//go:build windows

package dockerlog

import (
	"context"
	"net"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// npipeName is Docker Desktop's default engine socket on Windows.
const npipeName = `\\.\pipe\docker_engine`

// pipeConn adapts an os.File backed by a named pipe handle to net.Conn. The
// runtime poller does not drive deadlines on Windows named pipes, so deadline
// methods are no-ops: the dockerlog Client enforces its response bound by
// closing the connection when the context expires, which unblocks any Read.
type pipeConn struct {
	*os.File
}

func (c *pipeConn) LocalAddr() net.Addr              { return pipeAddr("local") }
func (c *pipeConn) RemoteAddr() net.Addr             { return pipeAddr("remote") }
func (c *pipeConn) SetDeadline(time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(time.Time) error { return nil }

type pipeAddr string

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return string(a) }

// DefaultDialer returns a dialer for Docker Desktop's named pipe.
func DefaultDialer() Dialer {
	return func(ctx context.Context) (net.Conn, error) {
		p, err := windows.UTF16PtrFromString(npipeName)
		if err != nil {
			return nil, err
		}
		h, err := windows.CreateFile(p,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if err != nil {
			return nil, err
		}
		return &pipeConn{File: os.NewFile(uintptr(h), npipeName)}, nil
	}
}
