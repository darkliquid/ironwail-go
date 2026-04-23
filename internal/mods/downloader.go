// Package mods implements the HTTP-backed addon downloader that
// mirrors C Ironwail's Modlist_* subsystem in host_cmd.c. It fetches
// an addons manifest (content.json) from a configurable URL, parses
// the available mods, and streams selected mods' pak archives into
// the user's install directory with background progress tracking.
//
// The package is deliberately small and UI-free: it provides the
// downloading/state primitives that the host exposes through
// cvars/commands and that the menu layer renders.
package mods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultAddonServer is the upstream default, matching
// DEFAULT_ADDON_SERVER in host_cmd.c.
const DefaultAddonServer = "https://kexquake.s3.amazonaws.com"

// ManifestFile is the content manifest filename appended to the
// addons URL. Matches ADDON_MANIFEST_FILE in host_cmd.c.
const ManifestFile = "content.json"

// ManifestRetention is the cache-freshness window for content.json.
// Matches MANIFEST_RETENTION in host_cmd.c.
const ManifestRetention = 24 * time.Hour

// ModStatus mirrors MODSTATUS_* enum values in host_cmd.c.
type ModStatus int32

const (
	StatusDownloadable ModStatus = iota
	StatusInstalling
	StatusInstalled
	StatusFailed
)

func (s ModStatus) String() string {
	switch s {
	case StatusDownloadable:
		return "downloadable"
	case StatusInstalling:
		return "installing"
	case StatusInstalled:
		return "installed"
	case StatusFailed:
		return "failed"
	}
	return "unknown"
}

// RemoteMod describes a single addon entry from the manifest.
type RemoteMod struct {
	Name        string  `json:"gamedir"`
	FullName    string  `json:"name"`
	Author      string  `json:"author"`
	Date        string  `json:"date"`
	Download    string  `json:"download"`
	Size        float64 `json:"size"`
	Description any     `json:"description"` // may be string or object
}

// LocalizedDescription returns the English description when the
// manifest uses the localized `{ "en": "..." }` form, or the raw
// string if a plain string was supplied.
func (m RemoteMod) LocalizedDescription() string {
	switch d := m.Description.(type) {
	case string:
		return d
	case map[string]any:
		if v, ok := d["en"].(string); ok {
			return v
		}
	}
	return ""
}

// Manifest is the decoded top-level content.json payload.
type Manifest struct {
	Addons []RemoteMod `json:"addons"`
}

// Event is a structured sysdbg-friendly notification emitted during
// manifest fetches and installs. Fields are populated based on Kind.
type Event struct {
	Kind    string
	Mod     string // addon gamedir, optional
	URL     string // target URL, optional
	HTTP    int    // HTTP status, optional
	Bytes   int64  // chunk or total bytes, optional
	Elapsed time.Duration
	Err     error
}

// InstallState tracks an in-progress installation. All fields are
// read with atomic loads from the callers' thread; writes happen on
// the installer goroutine.
type InstallState struct {
	Status          atomic.Int32
	BytesDownloaded atomic.Int64
	BytesTotal      int64
	Name            string
	DownloadURL     string
}

// GetStatus returns the current ModStatus.
func (s *InstallState) GetStatus() ModStatus {
	return ModStatus(s.Status.Load())
}

// Downloader is the addon downloader runtime. A zero value is not
// usable; create with New.
type Downloader struct {
	httpClient *http.Client
	baseURL    string
	cacheDir   string // base dir for addons.url.dat
	installDir string // where addon directories are created

	mu           sync.Mutex
	installing   map[string]*InstallState
	installOrder []string // FIFO of current installs (C allows only 1)

	jsonCancel    atomic.Bool
	installCancel atomic.Bool

	onEvent func(Event)
}

// Config configures a Downloader.
type Config struct {
	// BaseURL is the addons server root (no trailing slash). Defaults
	// to DefaultAddonServer when empty.
	BaseURL string
	// CacheDir is where the URL-change stamp (addons.url.dat) lives.
	CacheDir string
	// InstallDir is where <addon>/pak0.pak files are written.
	InstallDir string
	// HTTPClient overrides the default client (useful in tests).
	HTTPClient *http.Client
	// OnEvent is invoked for each progress milestone. Called from the
	// downloader goroutine; implementations should marshal to the
	// main thread (e.g. via host.InvokeOnMainThread) if they touch
	// engine state.
	OnEvent func(Event)
}

// New constructs a Downloader.
func New(cfg Config) *Downloader {
	d := &Downloader{
		httpClient: cfg.HTTPClient,
		baseURL:    cfg.BaseURL,
		cacheDir:   cfg.CacheDir,
		installDir: cfg.InstallDir,
		installing: make(map[string]*InstallState),
		onEvent:    cfg.OnEvent,
	}
	if d.httpClient == nil {
		d.httpClient = http.DefaultClient
	}
	if d.baseURL == "" {
		d.baseURL = DefaultAddonServer
	}
	return d
}

// BaseURL returns the currently-configured addons URL.
func (d *Downloader) BaseURL() string { return d.baseURL }

// SetBaseURL updates the addons URL. Running operations are not
// interrupted; future fetches use the new value.
func (d *Downloader) SetBaseURL(u string) {
	if u == "" {
		u = DefaultAddonServer
	}
	d.baseURL = u
}

// CancelManifestFetch flags any in-progress FetchManifest call to
// abort at the next chance.
func (d *Downloader) CancelManifestFetch() {
	d.jsonCancel.Store(true)
}

// CancelInstall flags any in-progress install to abort.
func (d *Downloader) CancelInstall() {
	d.installCancel.Store(true)
}

func (d *Downloader) emit(e Event) {
	if d.onEvent != nil {
		d.onEvent(e)
	}
}

// FetchManifest returns the parsed content.json, using the disk
// cache when present and fresh (and the URL has not changed since
// the cache was written), and falling back to HTTP otherwise.
//
// Cached files are written to <CacheDir>/addons.url.dat and
// <InstallDir>/addons.json, matching the C implementation's
// cache-key semantics.
func (d *Downloader) FetchManifest(ctx context.Context) (*Manifest, error) {
	d.jsonCancel.Store(false)
	manifestPath := filepath.Join(d.installDir, "addons.json")
	urlStampPath := filepath.Join(d.cacheDir, "addons.url.dat")

	urlUnchanged := false
	if b, err := os.ReadFile(urlStampPath); err == nil && string(b) == d.baseURL {
		urlUnchanged = true
	}

	if urlUnchanged && d.installDir != "" {
		if info, err := os.Stat(manifestPath); err == nil {
			if time.Since(info.ModTime()) < ManifestRetention {
				if data, err := os.ReadFile(manifestPath); err == nil {
					var m Manifest
					if err := json.Unmarshal(data, &m); err == nil {
						d.emit(Event{Kind: "manifest_cache", URL: d.baseURL, Bytes: int64(len(data))})
						return &m, nil
					}
				}
			}
		}
	}

	manifestURL, err := joinURL(d.baseURL, ManifestFile)
	if err != nil {
		return nil, fmt.Errorf("invalid addons URL: %w", err)
	}

	start := time.Now()
	d.emit(Event{Kind: "manifest_fetch_begin", URL: manifestURL})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.emit(Event{Kind: "manifest_fetch_error", URL: manifestURL, Err: err})
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("manifest HTTP %d", resp.StatusCode)
		d.emit(Event{Kind: "manifest_fetch_error", URL: manifestURL, HTTP: resp.StatusCode, Err: err})
		return nil, err
	}
	if d.jsonCancel.Load() {
		return nil, errors.New("manifest fetch cancelled")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		d.emit(Event{Kind: "manifest_fetch_error", URL: manifestURL, Err: err})
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		d.emit(Event{Kind: "manifest_parse_error", URL: manifestURL, Err: err})
		return nil, err
	}
	// Best-effort cache write.
	if d.installDir != "" {
		_ = os.MkdirAll(d.installDir, 0o755)
		_ = os.WriteFile(manifestPath, data, 0o644)
	}
	if d.cacheDir != "" {
		_ = os.MkdirAll(d.cacheDir, 0o755)
		_ = os.WriteFile(urlStampPath, []byte(d.baseURL), 0o644)
	}

	d.emit(Event{Kind: "manifest_fetch_done", URL: manifestURL, HTTP: resp.StatusCode, Bytes: int64(len(data)), Elapsed: time.Since(start)})
	return &m, nil
}

// IsInstalling returns true when any addon install is in progress.
// Mirrors Modlist_IsInstalling.
func (d *Downloader) IsInstalling() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.installing) > 0
}

// InstallStateOf returns the current install state for a given
// addon gamedir, or nil if none.
func (d *Downloader) InstallStateOf(name string) *InstallState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.installing[name]
}

// StartInstall kicks off a background install of the given addon.
// The C implementation permits only a single concurrent install; we
// preserve that constraint by returning nil when another install is
// already running. onDone is invoked on the installer goroutine with
// a nil error on success. Callers should marshal to the main thread
// via host.InvokeOnMainThread if onDone mutates engine state.
//
// The downloaded pak is written atomically: bytes stream to
// <installDir>/<name>/pak0.<suffix>.tmp, then os.Rename to pak0.pak.
func (d *Downloader) StartInstall(ctx context.Context, mod RemoteMod, onDone func(error)) (*InstallState, error) {
	if mod.Name == "" || mod.Download == "" {
		return nil, errors.New("mod missing name or download URL")
	}
	d.installCancel.Store(false)
	d.mu.Lock()
	if _, exists := d.installing[mod.Name]; exists {
		d.mu.Unlock()
		return nil, errors.New("install already in progress for this mod")
	}
	if len(d.installing) > 0 {
		d.mu.Unlock()
		return nil, errors.New("another install is in progress")
	}
	state := &InstallState{
		Name:        mod.Name,
		DownloadURL: mod.Download,
		BytesTotal:  int64(mod.Size),
	}
	state.Status.Store(int32(StatusInstalling))
	d.installing[mod.Name] = state
	d.installOrder = append(d.installOrder, mod.Name)
	d.mu.Unlock()

	go func() {
		err := d.runInstall(ctx, mod, state)
		d.mu.Lock()
		delete(d.installing, mod.Name)
		for i, n := range d.installOrder {
			if n == mod.Name {
				d.installOrder = append(d.installOrder[:i], d.installOrder[i+1:]...)
				break
			}
		}
		d.mu.Unlock()
		if err != nil {
			state.Status.Store(int32(StatusFailed))
		} else {
			state.Status.Store(int32(StatusInstalled))
		}
		if onDone != nil {
			onDone(err)
		}
	}()
	return state, nil
}

func (d *Downloader) runInstall(ctx context.Context, mod RemoteMod, state *InstallState) error {
	modDir := filepath.Join(d.installDir, mod.Name)
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		return err
	}
	tmpPath := filepath.Join(modDir, "pak0.download.tmp")
	finalPath := filepath.Join(modDir, "pak0.pak")

	modURL, err := joinURL(d.baseURL, mod.Download)
	if err != nil {
		return err
	}

	d.emit(Event{Kind: "install_begin", Mod: mod.Name, URL: modURL, Bytes: state.BytesTotal})
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modURL, nil)
	if err != nil {
		return err
	}
	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.emit(Event{Kind: "install_error", Mod: mod.Name, URL: modURL, Err: err})
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("install HTTP %d", resp.StatusCode)
		d.emit(Event{Kind: "install_error", Mod: mod.Name, URL: modURL, HTTP: resp.StatusCode, Err: err})
		return err
	}

	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	buf := make([]byte, 32*1024)
	for {
		if d.installCancel.Load() {
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return errors.New("install cancelled")
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				_ = file.Close()
				_ = os.Remove(tmpPath)
				return werr
			}
			state.BytesDownloaded.Add(int64(n))
			d.emit(Event{Kind: "install_chunk", Mod: mod.Name, Bytes: int64(n)})
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return rerr
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		d.emit(Event{Kind: "install_error", Mod: mod.Name, Err: err})
		return err
	}
	d.emit(Event{Kind: "install_done", Mod: mod.Name, URL: modURL, Bytes: state.BytesDownloaded.Load(), Elapsed: time.Since(start)})
	return nil
}

// joinURL joins a base URL with a relative path the way the C
// implementation expects: simple `%s/%s` concatenation with
// leading-slash normalization on the path.
func joinURL(base, path string) (string, error) {
	if _, err := url.Parse(base); err != nil {
		return "", err
	}
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	if len(base) > 0 && base[len(base)-1] == '/' {
		return base + path, nil
	}
	return base + "/" + path, nil
}
