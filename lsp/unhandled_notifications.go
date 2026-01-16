package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
)

type unhandledNotifLevel string

const (
	unhandledNotifOff   unhandledNotifLevel = "off"
	unhandledNotifDebug unhandledNotifLevel = "debug"
	unhandledNotifInfo  unhandledNotifLevel = "info"
)

type unhandledNotifConfig struct {
	level         unhandledNotifLevel
	window        time.Duration
	burstPerKey   int
	maxParamBytes int
}

type unhandledNotifBucket struct {
	windowStart time.Time
	emitted     int
	suppressed  int
	suppressMsg bool
}

var (
	unhandledNotifOnce sync.Once
	unhandledNotifCfg  unhandledNotifConfig

	unhandledNotifMu      sync.Mutex
	unhandledNotifBuckets = map[string]*unhandledNotifBucket{}
)

func loadUnhandledNotifConfig() unhandledNotifConfig {
	cfg := unhandledNotifConfig{
		level:         unhandledNotifDebug,
		window:        10 * time.Second,
		burstPerKey:   3,
		maxParamBytes: 4096,
	}

	if v := os.Getenv("MCP_LSP_UNHANDLED_NOTIFICATIONS_LEVEL"); v != "" {
		switch unhandledNotifLevel(v) {
		case unhandledNotifOff, unhandledNotifDebug, unhandledNotifInfo:
			cfg.level = unhandledNotifLevel(v)
		}
	}

	if v := os.Getenv("MCP_LSP_UNHANDLED_NOTIFICATIONS_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.window = d
		}
	}

	if v := os.Getenv("MCP_LSP_UNHANDLED_NOTIFICATIONS_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.burstPerKey = n
		}
	}

	if v := os.Getenv("MCP_LSP_UNHANDLED_NOTIFICATIONS_MAX_PARAM_BYTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.maxParamBytes = n
		}
	}

	return cfg
}

func logUnhandledNotification(method string, rawParams *json.RawMessage) {
	unhandledNotifOnce.Do(func() {
		unhandledNotifCfg = loadUnhandledNotifConfig()
	})

	cfg := unhandledNotifCfg
	if cfg.level == unhandledNotifOff {
		return
	}

	now := time.Now()

	unhandledNotifMu.Lock()
	b := unhandledNotifBuckets[method]
	if b == nil {
		b = &unhandledNotifBucket{windowStart: now}
		unhandledNotifBuckets[method] = b
	}

	// Window rollover: flush suppression summary and reset counters.
	if cfg.window > 0 && now.Sub(b.windowStart) >= cfg.window {
		if b.suppressed > 0 {
			msg := fmt.Sprintf("Unhandled notification suppressed: method=%s suppressed=%d window=%s", method, b.suppressed, cfg.window)
			unhandledNotifMu.Unlock()
			logUnhandledByLevel(cfg.level, msg)
			unhandledNotifMu.Lock()
		}
		b.windowStart = now
		b.emitted = 0
		b.suppressed = 0
		b.suppressMsg = false
	}

	// Rate-limit (per method).
	if cfg.burstPerKey == 0 || b.emitted >= cfg.burstPerKey {
		b.suppressed++
		needSuppressMsg := !b.suppressMsg && cfg.burstPerKey > 0
		if needSuppressMsg {
			b.suppressMsg = true
		}
		unhandledNotifMu.Unlock()

		if needSuppressMsg {
			logUnhandledByLevel(cfg.level, fmt.Sprintf("Unhandled notification flood detected: method=%s burst=%d window=%s (suppressing further)", method, cfg.burstPerKey, cfg.window))
		}
		return
	}

	b.emitted++
	unhandledNotifMu.Unlock()

	msg := fmt.Sprintf("Unhandled notification: %s", method)
	if rawParams != nil && len(*rawParams) > 0 && cfg.maxParamBytes != 0 {
		p := []byte(*rawParams)
		if cfg.maxParamBytes > 0 && len(p) > cfg.maxParamBytes {
			p = p[:cfg.maxParamBytes]
			msg = fmt.Sprintf("%s params=%s...(truncated)", msg, string(p))
		} else {
			msg = fmt.Sprintf("%s params=%s", msg, string(p))
		}
	} else if rawParams == nil || len(*rawParams) == 0 {
		msg = fmt.Sprintf("%s (no params)", msg)
	}

	logUnhandledByLevel(cfg.level, msg)
}

func logUnhandledByLevel(level unhandledNotifLevel, msg string) {
	switch level {
	case unhandledNotifInfo:
		logger.Info(msg)
	default:
		// default/debug
		logger.Debug(msg)
	}
}
