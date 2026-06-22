package xmail

import (
	"net"
	"time"
)

// dialTimeout is a small wrapper so smtp.go stays minimal. Kept in a
// separate file so it can be swapped in tests with a mock dialer.
func dialTimeout(network, addr string, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	return d.Dial(network, addr)
}
