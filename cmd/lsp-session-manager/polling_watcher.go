package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FileWatcherMode определяет режим отслеживания файлов
type FileWatcherMode string

const (
	// WatcherModeOff - отслеживание отключено (только ручной tool)
	WatcherModeOff FileWatcherMode = "off"
	// WatcherModePolling - polling-based отслеживание (для Docker на Windows)
	WatcherModePolling FileWatcherMode = "polling"
	// WatcherModeFsnotify - fsnotify-based отслеживание (для Linux native)
	WatcherModeFsnotify FileWatcherMode = "fsnotify"
	// WatcherModeAuto - автоматический выбор (fsnotify, fallback на polling)
	WatcherModeAuto FileWatcherMode = "auto"
)

// PollingWatcher отслеживает изменения файлов через периодическое сканирование
type PollingWatcher struct {
	workspaceDir   string
	extensions     []string // ".bsl", ".os"
	interval       time.Duration
	workers        int
	sendNotifyFunc func(changes []FileChange) error

	mu       sync.RWMutex
	fileMap  map[string]int64 // path -> mtime
	running  bool
	stopChan chan struct{}
}

// FileChange представляет изменение файла
type FileChange struct {
	URI  string `json:"uri"`
	Type int    `json:"type"` // 1=Created, 2=Changed, 3=Deleted
}

// NewPollingWatcher создаёт новый polling watcher
func NewPollingWatcher(workspaceDir string, interval time.Duration, workers int, notifyFunc func([]FileChange) error) *PollingWatcher {
	return &PollingWatcher{
		workspaceDir:   workspaceDir,
		extensions:     []string{".bsl", ".os"},
		interval:       interval,
		workers:        workers,
		sendNotifyFunc: notifyFunc,
		fileMap:        make(map[string]int64),
		stopChan:       make(chan struct{}),
	}
}

// Start запускает polling watcher
func (pw *PollingWatcher) Start() error {
	pw.mu.Lock()
	if pw.running {
		pw.mu.Unlock()
		return nil
	}
	pw.running = true
	pw.mu.Unlock()

	log.Printf("Polling watcher starting (interval: %v, workers: %d)", pw.interval, pw.workers)

	// Первоначальное сканирование
	start := time.Now()
	initialFiles := pw.scan()
	elapsed := time.Since(start)

	pw.mu.Lock()
	pw.fileMap = initialFiles
	pw.mu.Unlock()

	log.Printf("Polling watcher initial scan: %d files in %v", len(initialFiles), elapsed)

	// Запуск периодического сканирования
	go pw.runPollingLoop()

	return nil
}

// Stop останавливает polling watcher
func (pw *PollingWatcher) Stop() {
	pw.mu.Lock()
	if !pw.running {
		pw.mu.Unlock()
		return
	}
	pw.running = false
	pw.mu.Unlock()

	close(pw.stopChan)
	log.Println("Polling watcher stopped")
}

// runPollingLoop выполняет периодическое сканирование
func (pw *PollingWatcher) runPollingLoop() {
	ticker := time.NewTicker(pw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-pw.stopChan:
			return
		case <-ticker.C:
			pw.checkForChanges()
		}
	}
}

// checkForChanges сканирует и сравнивает с предыдущим состоянием
func (pw *PollingWatcher) checkForChanges() {
	start := time.Now()
	newFiles := pw.scan()
	scanTime := time.Since(start)

	pw.mu.Lock()
	oldFiles := pw.fileMap
	pw.fileMap = newFiles
	pw.mu.Unlock()

	// Сравниваем
	var changes []FileChange

	// Проверяем новые и изменённые файлы
	for path, newMtime := range newFiles {
		oldMtime, exists := oldFiles[path]
		if !exists {
			// Новый файл
			changes = append(changes, FileChange{
				URI:  pathToURI(path),
				Type: 1, // Created
			})
		} else if newMtime != oldMtime {
			// Изменённый файл
			changes = append(changes, FileChange{
				URI:  pathToURI(path),
				Type: 2, // Changed
			})
		}
	}

	// Проверяем удалённые файлы
	for path := range oldFiles {
		if _, exists := newFiles[path]; !exists {
			changes = append(changes, FileChange{
				URI:  pathToURI(path),
				Type: 3, // Deleted
			})
		}
	}

	if len(changes) > 0 {
		log.Printf("Polling watcher detected %d changes (scan: %v)", len(changes), scanTime)
		for _, c := range changes {
			changeType := "?"
			switch c.Type {
			case 1:
				changeType = "created"
			case 2:
				changeType = "changed"
			case 3:
				changeType = "deleted"
			}
			log.Printf("  %s: %s", changeType, c.URI)
		}

		if pw.sendNotifyFunc != nil {
			if err := pw.sendNotifyFunc(changes); err != nil {
				log.Printf("Error sending file changes notification: %v", err)
			}
		}
	}
}

// scan выполняет параллельное сканирование директории
func (pw *PollingWatcher) scan() map[string]int64 {
	result := make(map[string]int64)
	resultMu := sync.Mutex{}

	dirs := make(chan string, 1000)
	var wg sync.WaitGroup
	var activeWorkers int32

	// Запускаем воркеры
	for i := 0; i < pw.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dir := range dirs {
				atomic.AddInt32(&activeWorkers, 1)
				pw.processDir(dir, &result, &resultMu, dirs)
				atomic.AddInt32(&activeWorkers, -1)
			}
		}()
	}

	// Начинаем с корневой директории
	dirs <- pw.workspaceDir

	// Ждём завершения
	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			if len(dirs) == 0 && atomic.LoadInt32(&activeWorkers) == 0 {
				close(dirs)
				return
			}
		}
	}()

	wg.Wait()
	return result
}

// processDir обрабатывает одну директорию
func (pw *PollingWatcher) processDir(dir string, result *map[string]int64, resultMu *sync.Mutex, dirs chan string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)

		if entry.IsDir() {
			// Пропускаем скрытые и служебные директории
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				continue
			}
			// Отправляем в очередь или обрабатываем inline
			select {
			case dirs <- path:
			default:
				pw.processDir(path, result, resultMu, dirs)
			}
		} else {
			// Проверяем расширение
			ext := strings.ToLower(filepath.Ext(name))
			isTarget := false
			for _, targetExt := range pw.extensions {
				if ext == targetExt {
					isTarget = true
					break
				}
			}

			if isTarget {
				info, err := entry.Info()
				if err == nil {
					resultMu.Lock()
					(*result)[path] = info.ModTime().Unix()
					resultMu.Unlock()
				}
			}
		}
	}
}

// pathToURI конвертирует путь в file:// URI
func pathToURI(path string) string {
	// Нормализуем путь
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}

// GetFileWatcherMode возвращает режим из переменной окружения
func GetFileWatcherMode() FileWatcherMode {
	mode := os.Getenv("FILE_WATCHER_MODE")
	switch strings.ToLower(mode) {
	case "off", "manual", "disabled":
		return WatcherModeOff
	case "polling", "poll":
		return WatcherModePolling
	case "fsnotify", "inotify", "native":
		return WatcherModeFsnotify
	case "auto", "":
		return WatcherModeAuto
	default:
		log.Printf("Unknown FILE_WATCHER_MODE '%s', using 'auto'", mode)
		return WatcherModeAuto
	}
}

// GetPollingInterval возвращает интервал polling из переменной окружения
func GetPollingInterval() time.Duration {
	intervalStr := os.Getenv("FILE_WATCHER_INTERVAL")
	if intervalStr == "" {
		return 30 * time.Second // default
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Printf("Invalid FILE_WATCHER_INTERVAL '%s', using 30s", intervalStr)
		return 30 * time.Second
	}
	return interval
}

// GetPollingWorkers возвращает количество воркеров из переменной окружения
func GetPollingWorkers() int {
	workersStr := os.Getenv("FILE_WATCHER_WORKERS")
	if workersStr == "" {
		return 8 // default
	}
	workers, err := strconv.Atoi(workersStr)
	if err != nil || workers < 1 {
		log.Printf("Invalid FILE_WATCHER_WORKERS '%s', using 8", workersStr)
		return 8
	}
	return workers
}
