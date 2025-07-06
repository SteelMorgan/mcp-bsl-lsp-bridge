package types

import "time"

// type ClientStatusEnum int
//
// const (
// 	StatusUninitialized ClientStatusEnum = iota
// 	StatusConnecting
// 	StatusConnected
// 	StatusError
// 	StatusRestarting
// 	StatusDisconnected
// )

type ClientStatusProvider interface {
	Status() int
	String() string
}

type ClientMetricsProvider interface {
	GetCommand() string
	SetCommand(command string)

	GetStatus() int
	SetStatus(status int)

	GetTotalRequests() int64
	SetTotalRequests(total int64)
	IncrementTotalRequests()

	GetSuccessfulRequests() int64
	SetSuccessfulRequests(successful int64)
	IncrementSuccessfulRequests()

	GetFailedRequests() int64
	SetFailedRequests(failed int64)
	IncrementFailedRequests()

	GetLastInitialized() time.Time
	SetLastInitialized(t time.Time)

	GetLastErrorTime() time.Time
	SetLastErrorTime(t time.Time)

	GetLastError() string
	SetLastError(err string)

	IsConnected() bool
	SetConnected(connected bool)

	GetProcessID() int32
	SetProcessID(pid int32)
}
