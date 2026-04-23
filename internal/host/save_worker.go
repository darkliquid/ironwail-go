// Background save worker mirroring Host_BackgroundSave /
// Host_InitSaveThread / Host_WaitForSaveThread / Host_IsSaving from
// C Ironwail's host_cmd.c. The main thread remains responsible for
// capturing the savegame state (which touches live server/client
// structures) and marshaling it to bytes; the resulting byte buffer
// and file I/O are handed off to a goroutine so the simulation does
// not stall waiting for disk. Completion callbacks are marshaled
// back to the main thread via Host.MainThreadQueue.

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// saveWorker tracks in-flight background save writes so shutdown and
// subsequent load operations can wait for pending disk I/O.
type saveWorker struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	inflight map[string]int // normalized save name -> outstanding writes
	active   atomic.Int32
}

func (s *saveWorker) begin(name string) {
	s.mu.Lock()
	if s.inflight == nil {
		s.inflight = make(map[string]int, 2)
	}
	s.inflight[name]++
	s.mu.Unlock()
	s.active.Add(1)
	s.wg.Add(1)
}

func (s *saveWorker) end(name string) {
	s.mu.Lock()
	if s.inflight != nil {
		if n := s.inflight[name] - 1; n <= 0 {
			delete(s.inflight, name)
		} else {
			s.inflight[name] = n
		}
	}
	s.mu.Unlock()
	s.active.Add(-1)
	s.wg.Done()
}

func (s *saveWorker) isSavingName(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inflight[name] > 0
}

// IsSaving reports whether any background save is currently writing
// to disk. Mirrors Host_IsSaving from host_cmd.c.
func (h *Host) IsSaving() bool {
	if h == nil {
		return false
	}
	return h.saveWorker.active.Load() > 0
}

// IsSavingName reports whether a background save for the given slot
// name is currently in flight.
func (h *Host) IsSavingName(name string) bool {
	if h == nil {
		return false
	}
	rel, err := normalizeSaveName(name)
	if err != nil {
		return false
	}
	return h.saveWorker.isSavingName(rel)
}

// WaitForSaveThread blocks until all outstanding background save
// writes have completed. Call this before a blocking load of a slot
// that may still be mid-write, and during host shutdown. Mirrors
// Host_WaitForSaveThread.
func (h *Host) WaitForSaveThread() {
	if h == nil {
		return
	}
	h.saveWorker.wg.Wait()
}

// writeSaveInBackground spawns a goroutine that performs the
// MkdirAll + WriteFile for a previously-captured save payload. On
// completion (success or failure) a callback is marshaled back onto
// the main thread to surface the result via the console and clear
// the saving indicator.
func (h *Host) writeSaveInBackground(relName, path, displayName string, data []byte, subs *Subsystems, skipNotify bool) {
	h.saveWorker.begin(relName)
	start := time.Now()
	hostDebugSysLogf("savegame", "begin slot=%q path=%q bytes=%d", relName, path, len(data))

	go func() {
		defer h.saveWorker.end(relName)
		var writeErr error
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			writeErr = err
		} else if err := os.WriteFile(path, data, 0o644); err != nil {
			writeErr = err
		}
		elapsed := time.Since(start)
		payloadBytes := len(data)

		cb := func() {
			if writeErr != nil {
				hostDebugSysLogf("savegame", "failed slot=%q path=%q err=%v elapsed_ms=%.3f", relName, path, writeErr, float64(elapsed.Microseconds())/1000)
				if subs != nil && subs.Console != nil {
					subs.Console.Print("ERROR: couldn't open.\n")
				}
				return
			}
			hostDebugSysLogf("savegame", "done slot=%q bytes=%d elapsed_ms=%.3f", relName, payloadBytes, float64(elapsed.Microseconds())/1000)
			if !skipNotify && subs != nil && subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("Saving game to %s...\n", displayName))
			}
		}
		// InvokeOnMainThread marshals cb onto Host.Frame's goroutine.
		// If the host has already been shut down (queue closed) we
		// fall back to executing cb immediately so errors still log.
		if !h.InvokeOnMainThread(cb) {
			cb()
		}
	}()
}
