package lsp

import (
	"rockerboo/mcp-lsp-bridge/types"
	"time"
)

// ClientMetrics implements the ClientMetricsProvider interface
type ClientMetrics struct {
	Command            string    `json:"command"`
	Status             int       `json:"status"`
	TotalRequests      int64     `json:"total_requests"`
	SuccessfulRequests int64     `json:"successful_requests"`
	FailedRequests     int64     `json:"failed_requests"`
	LastInitialized    time.Time `json:"last_initialized"`
	LastErrorTime      time.Time `json:"last_error_time"`
	LastError          string    `json:"last_error,omitempty"`
	Connected          bool      `json:"is_connected"`
	ProcessID          int32     `json:"process_id"`
}

// Command methods
func (c *ClientMetrics) GetCommand() string {
	return c.Command
}

func (c *ClientMetrics) SetCommand(command string) {
	c.Command = command
}

// Status methods
func (c *ClientMetrics) GetStatus() int {
	return c.Status
}

func (c *ClientMetrics) SetStatus(status int) {
	c.Status = status
}

// TotalRequests methods
func (c *ClientMetrics) GetTotalRequests() int64 {
	return c.TotalRequests
}

func (c *ClientMetrics) SetTotalRequests(total int64) {
	c.TotalRequests = total
}

func (c *ClientMetrics) IncrementTotalRequests() {
	c.TotalRequests++
}

// SuccessfulRequests methods
func (c *ClientMetrics) GetSuccessfulRequests() int64 {
	return c.SuccessfulRequests
}

func (c *ClientMetrics) SetSuccessfulRequests(successful int64) {
	c.SuccessfulRequests = successful
}

func (c *ClientMetrics) IncrementSuccessfulRequests() {
	c.SuccessfulRequests++
}

// FailedRequests methods
func (c *ClientMetrics) GetFailedRequests() int64 {
	return c.FailedRequests
}

func (c *ClientMetrics) SetFailedRequests(failed int64) {
	c.FailedRequests = failed
}

func (c *ClientMetrics) IncrementFailedRequests() {
	c.FailedRequests++
}

// LastInitialized methods
func (c *ClientMetrics) GetLastInitialized() time.Time {
	return c.LastInitialized
}

func (c *ClientMetrics) SetLastInitialized(t time.Time) {
	c.LastInitialized = t
}

// LastErrorTime methods
func (c *ClientMetrics) GetLastErrorTime() time.Time {
	return c.LastErrorTime
}

func (c *ClientMetrics) SetLastErrorTime(t time.Time) {
	c.LastErrorTime = t
}

// LastError methods
func (c *ClientMetrics) GetLastError() string {
	return c.LastError
}

func (c *ClientMetrics) SetLastError(err string) {
	c.LastError = err
}

// Connected methods
func (c *ClientMetrics) IsConnected() bool {
	return c.Connected
}

func (c *ClientMetrics) SetConnected(connected bool) {
	c.Connected = connected
}

// ProcessID methods
func (c *ClientMetrics) GetProcessID() int32 {
	return c.ProcessID
}

func (c *ClientMetrics) SetProcessID(pid int32) {
	c.ProcessID = pid
}

// NewClientMetrics creates a new ClientMetricsProvider instance
func NewClientMetrics() types.ClientMetricsProvider {
	return &ClientMetrics{
		LastInitialized: time.Now(),
	}
}
