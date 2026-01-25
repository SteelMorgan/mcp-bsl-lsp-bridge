//go:build ignore
// +build ignore

// CPU benchmark for file scanning
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	root := "/projects/test-workspace"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	workers := 8
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &workers)
	}

	fmt.Printf("CPU Benchmark: %s with %d workers\n", root, workers)
	fmt.Printf("Available CPUs: %d\n\n", runtime.NumCPU())

	// Warm up
	parallelWalkWithMtime(root, workers)

	// Run 5 iterations and measure
	var totalTime time.Duration
	var totalFiles int64

	for i := 0; i < 5; i++ {
		// Get CPU stats before
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		start := time.Now()
		files := parallelWalkWithMtime(root, workers)
		elapsed := time.Since(start)

		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		totalTime += elapsed
		totalFiles = files

		fmt.Printf("Run %d: %d files in %v (alloc: %d KB)\n",
			i+1, files, elapsed, (m2.TotalAlloc-m1.TotalAlloc)/1024)
	}

	avgTime := totalTime / 5
	fmt.Printf("\nAverage: %v for %d files\n", avgTime, totalFiles)
	fmt.Printf("Rate: %.1f files/sec\n", float64(totalFiles)/avgTime.Seconds())

	// CPU utilization estimate
	// During scan, measure goroutine activity
	fmt.Printf("\n--- CPU Utilization Test (10 sec scan loop) ---\n")

	done := make(chan bool)
	var scanCount int32

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				parallelWalkWithMtime(root, workers)
				atomic.AddInt32(&scanCount, 1)
			}
		}
	}()

	time.Sleep(10 * time.Second)
	close(done)

	scans := atomic.LoadInt32(&scanCount)
	fmt.Printf("Completed %d full scans in 10 seconds\n", scans)
	fmt.Printf("Average scan time: %.2f seconds\n", 10.0/float64(scans))
	fmt.Printf("Duty cycle if polling every 30s: %.1f%%\n", (10.0/float64(scans))/30*100)
}

type fileInfo struct {
	path  string
	mtime int64
}

func parallelWalkWithMtime(root string, workers int) int64 {
	var count int64
	results := make(chan fileInfo, 1000)
	dirs := make(chan string, 1000)
	var wg sync.WaitGroup

	// Collector goroutine
	fileMap := make(map[string]int64)
	var mapMu sync.Mutex
	collectorDone := make(chan bool)

	go func() {
		for fi := range results {
			mapMu.Lock()
			fileMap[fi.path] = fi.mtime
			mapMu.Unlock()
		}
		close(collectorDone)
	}()

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dir := range dirs {
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, entry := range entries {
					path := filepath.Join(dir, entry.Name())
					if entry.IsDir() {
						if !strings.HasPrefix(entry.Name(), ".") {
							select {
							case dirs <- path:
							default:
								processWithMtime(path, results, dirs)
							}
						}
					} else if strings.HasSuffix(entry.Name(), ".bsl") || strings.HasSuffix(entry.Name(), ".os") {
						info, err := entry.Info()
						if err == nil {
							atomic.AddInt64(&count, 1)
							results <- fileInfo{path: path, mtime: info.ModTime().Unix()}
						}
					}
				}
			}
		}()
	}

	dirs <- root

	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			if len(dirs) == 0 {
				close(dirs)
				return
			}
		}
	}()

	wg.Wait()
	close(results)
	<-collectorDone

	return count
}

func processWithMtime(dir string, results chan fileInfo, dirs chan string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if !strings.HasPrefix(entry.Name(), ".") {
				select {
				case dirs <- path:
				default:
					processWithMtime(path, results, dirs)
				}
			}
		} else if strings.HasSuffix(entry.Name(), ".bsl") || strings.HasSuffix(entry.Name(), ".os") {
			info, err := entry.Info()
			if err == nil {
				results <- fileInfo{path: path, mtime: info.ModTime().Unix()}
			}
		}
	}
}
