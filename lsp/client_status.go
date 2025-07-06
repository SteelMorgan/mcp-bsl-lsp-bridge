package lsp

// ClientStatus represents the current status of a language client
type ClientStatus int

const (
	StatusUninitialized ClientStatus = iota
	StatusConnecting
	StatusConnected
	StatusError
	StatusRestarting
	StatusDisconnected
)

func (cs ClientStatus) Status() int {
	return int(cs)
}

func (cs ClientStatus) String() string {
	switch cs {
	case StatusUninitialized:
		return "uninitialized"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusError:
		return "error"
	case StatusRestarting:
		return "restarting"
	case StatusDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}
