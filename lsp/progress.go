package lsp

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// ProgressEvent is a normalized view of $/progress payloads.
type ProgressEvent struct {
	TokenKey    string
	Kind        string // begin|report|end|unknown
	Title       string
	Message     string
	Percentage  *uint32
	Cancellable *bool
	Time        time.Time
	Raw         json.RawMessage
}

// ProgressSnapshot is returned to status tooling.
type ProgressSnapshot struct {
	Active        []ProgressEvent
	LastEvent     *ProgressEvent
	LastEventTime time.Time
}

// ProgressTracker tracks server-initiated workDone progress streams.
// It is fed by notifications like $/progress.
type ProgressTracker struct {
	mu     sync.RWMutex
	active map[string]ProgressEvent
	last   *ProgressEvent
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		active: make(map[string]ProgressEvent),
	}
}

func progressTokenKey(t protocol.ProgressToken) string {
	switch v := t.Value.(type) {
	case int32:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func (pt *ProgressTracker) RegisterToken(token protocol.ProgressToken) string {
	key := progressTokenKey(token)
	pt.mu.Lock()
	defer pt.mu.Unlock()
	// no-op; existence in active is driven by begin/report/end
	return key
}

func (pt *ProgressTracker) Update(params protocol.ProgressParams) {
	now := time.Now()
	key := progressTokenKey(params.Token)

	raw, err := json.Marshal(params.Value)
	if err != nil {
		// If we can't marshal, we still keep a marker event.
		ev := ProgressEvent{
			TokenKey: key,
			Kind:     "unknown",
			Time:     now,
		}
		pt.mu.Lock()
		pt.last = &ev
		pt.mu.Unlock()
		return
	}

	// Minimal decode common fields across begin/report/end
	var base struct {
		Kind        string  `json:"kind"`
		Title       string  `json:"title,omitempty"`
		Message     string  `json:"message,omitempty"`
		Percentage  *uint32 `json:"percentage,omitempty"`
		Cancellable *bool   `json:"cancellable,omitempty"`
	}
	_ = json.Unmarshal(raw, &base)

	ev := ProgressEvent{
		TokenKey:    key,
		Kind:        base.Kind,
		Title:       base.Title,
		Message:     base.Message,
		Percentage:  base.Percentage,
		Cancellable: base.Cancellable,
		Time:        now,
		Raw:         raw,
	}
	if ev.Kind == "" {
		ev.Kind = "unknown"
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.last = &ev

	switch ev.Kind {
	case "begin", "report":
		// Keep most recent event per token.
		pt.active[key] = ev
	case "end":
		delete(pt.active, key)
	default:
		// Keep it in active only if we already had it.
		if _, ok := pt.active[key]; ok {
			pt.active[key] = ev
		}
	}
}

func (pt *ProgressTracker) Snapshot() ProgressSnapshot {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	active := make([]ProgressEvent, 0, len(pt.active))
	for _, ev := range pt.active {
		active = append(active, ev)
	}

	var lastCopy *ProgressEvent
	var lastTime time.Time
	if pt.last != nil {
		tmp := *pt.last
		lastCopy = &tmp
		lastTime = tmp.Time
	}

	return ProgressSnapshot{
		Active:        active,
		LastEvent:     lastCopy,
		LastEventTime: lastTime,
	}
}
