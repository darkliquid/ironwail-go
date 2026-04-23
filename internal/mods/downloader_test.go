package mods

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchManifestHTTP(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+ManifestFile {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "addons": [
    {
      "gamedir": "hipnotic",
      "name": "Scourge of Armagon",
      "author": "Ritual",
      "date": "1997",
      "download": "mods/hipnotic.pak",
      "size": 12345,
      "description": {"en": "Mission Pack 1"}
    }
  ]
}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{
		BaseURL:    srv.URL,
		CacheDir:   dir,
		InstallDir: dir,
	})
	m, err := d.FetchManifest(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(m.Addons) != 1 || m.Addons[0].Name != "hipnotic" {
		t.Fatalf("bad manifest: %+v", m)
	}
	if d := m.Addons[0].LocalizedDescription(); d != "Mission Pack 1" {
		t.Fatalf("bad description: %q", d)
	}
	if hits != 1 {
		t.Fatalf("expected 1 HTTP hit, got %d", hits)
	}
	// Second fetch should hit cache (no new HTTP hits).
	if _, err := d.FetchManifest(context.Background()); err != nil {
		t.Fatalf("refetch: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected cache reuse, got %d hits", hits)
	}
}

func TestFetchManifestInvalidatesWhenURLChanges(t *testing.T) {
	dir := t.TempDir()
	// Prime cache with initial server.
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"addons":[]}`))
	}))
	defer srv1.Close()
	d := New(Config{BaseURL: srv1.URL, CacheDir: dir, InstallDir: dir})
	if _, err := d.FetchManifest(context.Background()); err != nil {
		t.Fatal(err)
	}
	// New URL should bypass cache.
	var hits int
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"addons":[]}`))
	}))
	defer srv2.Close()
	d.SetBaseURL(srv2.URL)
	if _, err := d.FetchManifest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("url change should bypass cache; got %d hits", hits)
	}
}

func TestFetchManifestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	var events []Event
	d := New(Config{BaseURL: srv.URL, CacheDir: t.TempDir(), InstallDir: t.TempDir(),
		OnEvent: func(e Event) { events = append(events, e) }})
	if _, err := d.FetchManifest(context.Background()); err == nil {
		t.Fatal("expected error on 500")
	}
	found := false
	for _, e := range events {
		if e.Kind == "manifest_fetch_error" && e.HTTP == 500 {
			found = true
		}
	}
	if !found {
		t.Fatalf("no manifest_fetch_error event: %+v", events)
	}
}

func TestStartInstallStreamsToDisk(t *testing.T) {
	payload := make([]byte, 128*1024)
	for i := range payload {
		payload[i] = byte(i)
	}
	var installHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mods/hipnotic.pak" {
			installHits++
			_, _ = w.Write(payload)
			return
		}
		t.Fatalf("unexpected path %q", r.URL.Path)
	}))
	defer srv.Close()

	installDir := t.TempDir()
	var chunkBytes int64
	var doneFired bool
	d := New(Config{
		BaseURL:    srv.URL,
		InstallDir: installDir,
		CacheDir:   t.TempDir(),
		OnEvent: func(e Event) {
			switch e.Kind {
			case "install_chunk":
				chunkBytes += e.Bytes
			case "install_done":
				doneFired = true
			}
		},
	})
	mod := RemoteMod{Name: "hipnotic", Download: "mods/hipnotic.pak", Size: float64(len(payload))}
	done := make(chan error, 1)
	_, err := d.StartInstall(context.Background(), mod, func(err error) { done <- err })
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("install err: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("install timed out")
	}
	if installHits != 1 {
		t.Fatalf("expected 1 install HTTP hit, got %d", installHits)
	}
	if !doneFired {
		t.Fatal("install_done event missing")
	}
	if chunkBytes != int64(len(payload)) {
		t.Fatalf("bytes mismatch: chunks=%d expected=%d", chunkBytes, len(payload))
	}
	got, err := os.ReadFile(filepath.Join(installDir, "hipnotic", "pak0.pak"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) || got[0] != payload[0] {
		t.Fatal("pak contents mismatch")
	}
	// Temp file must be cleaned up.
	if _, err := os.Stat(filepath.Join(installDir, "hipnotic", "pak0.download.tmp")); err == nil {
		t.Fatal("leftover tmp file")
	}
	if d.IsInstalling() {
		t.Fatal("IsInstalling should be false after completion")
	}
	if st := d.InstallStateOf("hipnotic"); st != nil {
		t.Fatal("InstallStateOf should clear after completion")
	}
}

func TestStartInstallSerializesConcurrentRequests(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	d := New(Config{BaseURL: srv.URL, InstallDir: t.TempDir(), CacheDir: t.TempDir()})
	done1 := make(chan error, 1)
	if _, err := d.StartInstall(context.Background(), RemoteMod{Name: "a", Download: "a.pak"}, func(err error) { done1 <- err }); err != nil {
		t.Fatal(err)
	}
	if _, err := d.StartInstall(context.Background(), RemoteMod{Name: "b", Download: "b.pak"}, nil); err == nil {
		t.Fatal("second concurrent install should be rejected")
	}
	close(block)
	<-done1
}

func TestJoinURL(t *testing.T) {
	for _, tc := range []struct{ base, path, want string }{
		{"https://example.com", "content.json", "https://example.com/content.json"},
		{"https://example.com/", "content.json", "https://example.com/content.json"},
		{"https://example.com", "/mods/a.pak", "https://example.com/mods/a.pak"},
	} {
		got, err := joinURL(tc.base, tc.path)
		if err != nil {
			t.Fatalf("%s+%s: %v", tc.base, tc.path, err)
		}
		if got != tc.want {
			t.Fatalf("%s+%s: got %q want %q", tc.base, tc.path, got, tc.want)
		}
	}
}
