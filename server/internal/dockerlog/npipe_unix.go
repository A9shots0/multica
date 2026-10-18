//go:build !windows

package dockerlog

import (
	"context"
	"net"
)

// unixSocket is the Docker Engine's default socket on Linux and macOS hosts.
const unixSocket = "/var/run/docker.sock"

// DefaultDialer returns a dialer for the engine's unix socket. macOS Docker
// Desktop also exposes it (via a socket symlink), and the compose deployment
// this daemon supervises runs on the same host, so the daemon user usually
// holds group membership on the socket.
func DefaultDialer() Dialer {
	return func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", unixSocket)
	}
}
