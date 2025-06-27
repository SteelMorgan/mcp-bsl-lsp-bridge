package lsp

import "io"

// stdioReadWriteCloser implements io.ReadWriteCloser
type stdioReadWriteCloser struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (rwc *stdioReadWriteCloser) Read(p []byte) (n int, err error) {
	return rwc.stdout.Read(p)
}

func (rwc *stdioReadWriteCloser) Write(p []byte) (n int, err error) {
	return rwc.stdin.Write(p)
}

func (rwc *stdioReadWriteCloser) Close() error {
	err1 := rwc.stdin.Close()
	err2 := rwc.stdout.Close()

	if err1 != nil {
		return err1
	}

	return err2
}
