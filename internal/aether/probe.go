package aether

import (
	"net"
	"strconv"
	"time"
)

// waitForListener returns true once something accepts a TCP connection on the
// loopback SOCKS port, or false if nothing does within timeout. It is a liveness
// probe only: it opens and closes a connection without speaking SOCKS, so it does
// not consume an endpoint or disturb Aether's own dialing.
func waitForListener(port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(Loopback, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}
