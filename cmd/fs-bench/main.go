// Benchmark: parallel filesystem scanning
package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	fmt.Printf("Scanning: %s\n\n", root)

	// Benchmark 1: Sequential walk
	start := time.Now()
	count1 := sequentialWalk(root)
	elapsed1 := time.Since(start)
	fmt.Printf("Sequential walk: %d files in %v (%.1f files/sec)\n", count1, elapsed1, float64(count1)/elapsed1.Seconds())

	// Benchmark 2: Parallel walk (8 workers)
	start = time.Now()
	count2 := parallelWalk(root, 8)
	elapsed2 := time.Since(start)
	fmt.Printf("Parallel walk (8 workers): %d files in %v (%.1f files/sec)\n", count2, elapsed2, float64(count2)/elapsed2.Seconds())

	// Benchmark 3: Parallel walk (32 workers)
	start = time.Now()
	count3 := parallelWalk(root, 32)
	elapsed3 := time.Since(start)
	fmt.Printf("Parallel walk (32 workers): %d files in %v (%.1f files/sec)\n", count3, elapsed3, float64(count3)/elapsed3.Seconds())

	// Benchmark 4: Parallel walk (64 workers)
	start = time.Now()
	count4 := parallelWalk(root, 64)
	elapsed4 := time.Since(start)
	fmt.Printf("Parallel walk (64 workers): %d files in %v (%.1f files/sec)\n", count4, elapsed4, float64(count4)/elapsed4.Seconds())

	// Benchmark 5: Only readdir (no stat)
	start = time.Now()
	count5 := parallelReaddir(root, 32)
	elapsed5 := time.Since(start)
	fmt.Printf("Parallel readdir only (32 workers): %d entries in %v\n", count5, elapsed5)

	fmt.Printf("\nSpeedup: %.2fx (sequential vs 32 workers)\n", elapsed1.Seconds()/elapsed2.Seconds())
}

func sequentialWalk(root string) int {
	count := 0
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".bsl") || strings.HasSuffix(path, ".os")) {
			count++
		}
		return nil
	})
	return count
}

func parallelWalk(root string, workers int) int {
	var count int64
	dirs := make(chan string, 1000)
	var wg sync.WaitGroup

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
						// Non-blocking send - if channel full, process inline
						select {
						case dirs <- path:
						default:
							// Process subdirectory inline
							processDir(path, &count, dirs)
						}
					} else if strings.HasSuffix(entry.Name(), ".bsl") || strings.HasSuffix(entry.Name(), ".os") {
						atomic.AddInt64(&count, 1)
					}
				}
			}
		}()
	}

	// Seed with root directory
	dirs <- root

	// Wait for some initial work, then close when done
	// Simple approach: wait until no new work for 100ms
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if len(dirs) == 0 {
				close(dirs)
				return
			}
		}
	}()

	wg.Wait()
	return int(count)
}

func processDir(dir string, count *int64, dirs chan string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			select {
			case dirs <- path:
			default:
				processDir(path, count, dirs)
			}
		} else if strings.HasSuffix(entry.Name(), ".bsl") || strings.HasSuffix(entry.Name(), ".os") {
			atomic.AddInt64(count, 1)
		}
	}
}

func parallelReaddir(root string, workers int) int {
	var count int64
	dirs := make(chan string, 1000)
	var wg sync.WaitGroup

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
					atomic.AddInt64(&count, 1)
					if entry.IsDir() {
						select {
						case dirs <- filepath.Join(dir, entry.Name()):
						default:
						}
					}
				}
			}
		}()
	}

	dirs <- root

	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if len(dirs) == 0 {
				close(dirs)
				return
			}
		}
	}()

	wg.Wait()
	return int(count)
}
