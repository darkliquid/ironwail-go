// Command qbspprof runs a qbsp compile under Go CPU/heap profilers for
// performance research (bead ironwail-go-ros).
//
// -deadline lets it dump profiles from a compile that never finishes
// (the superlinear maps): run Compile in a goroutine, and after the
// deadline flush CPU/heap profiles and exit. A partial profile of a
// still-running superlinear compile is representative of the hot path.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"time"

	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

func main() {
	cpuprof := flag.String("cpuprofile", "", "cpu profile output path")
	memprof := flag.String("memprofile", "", "mem profile output path")
	deadline := flag.Duration("deadline", 0, "dump partial profiles after this duration and exit (0 = wait for compile)")
	memlimit := flag.Int64("memlimit", 0, "soft GC memory limit in MiB (debug.SetMemoryLimit; 0 = off)")
	flag.Parse()

	if *memlimit > 0 {
		debug.SetMemoryLimit(*memlimit << 20)
	}

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: qbspprof [-cpuprofile f] [-memprofile f] [-deadline d] map.map")
		os.Exit(2)
	}
	mapPath := flag.Arg(0)

	if *cpuprof != "" {
		f, err := os.Create(*cpuprof)
		if err != nil {
			log.Fatalf("qbspprof: %v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("qbspprof: %v", err)
		}
	}

	f, err := os.Open(mapPath)
	if err != nil {
		log.Fatalf("qbspprof: %v", err)
	}
	m, err := qbsp.ParseMap(f)
	_ = f.Close()
	if err != nil {
		log.Fatalf("qbspprof: parse %s: %v", mapPath, err)
	}

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		_, cerr := qbsp.Compile(m, qbsp.Options{
			Log: func(f string, a ...any) { fmt.Printf("  "+f+"\n", a...) },
		})
		done <- cerr
	}()

	var timedOut bool
	if *deadline > 0 {
		select {
		case <-done:
		case <-time.After(*deadline):
			timedOut = true
			log.Printf("qbspprof: deadline %s hit, compile still running; dumping partial profiles", *deadline)
		}
	} else {
		if cerr := <-done; cerr != nil {
			log.Printf("qbspprof: compile error: %v", cerr)
		}
	}

	if *cpuprof != "" {
		pprof.StopCPUProfile()
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("---- qbspprof / ironwail-go ----\n")
	fmt.Printf("map: %s\n", mapPath)
	fmt.Printf("elapsed: %s completed: %v\n", time.Since(start).Truncate(time.Millisecond), !timedOut)
	fmt.Printf("mem: total-alloc=%d MiB sys=%d MiB gc-cycles=%d gc-pause-total=%d ms\n", ms.TotalAlloc>>20, ms.Sys>>20, ms.NumGC, ms.PauseTotalNs/1e6)

	if *memprof != "" {
		mf, err := os.Create(*memprof)
		if err != nil {
			log.Fatalf("qbspprof: %v", err)
		}
		if !timedOut {
			runtime.GC()
		}
		if err := pprof.WriteHeapProfile(mf); err != nil {
			log.Fatalf("qbspprof: %v", err)
		}
		_ = mf.Close()
	}
}
