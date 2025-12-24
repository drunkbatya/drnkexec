package tlsdial

// #cgo LDFLAGS: -lssl -lcrypto
// #include "tls_conn.h"
// #include <stdlib.h>
import "C"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
	"unsafe"
)

type Conn struct {
	c          *C.nrpe_ssl_conn
	remoteAddr net.Addr
	localAddr  net.Addr
	closed     bool
}

type staticAddr struct {
	network string
	addr    string
}

func (a staticAddr) Network() string { return a.network }
func (a staticAddr) String() string  { return a.addr }

func Dial(ctx context.Context, addr string, timeout time.Duration) (*Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, context.DeadlineExceeded
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	chost := C.CString(host)
	cport := C.CString(port)
	defer C.free(unsafe.Pointer(chost))
	defer C.free(unsafe.Pointer(cport))
	var cconn *C.nrpe_ssl_conn
	timeoutMs := C.long(timeout.Milliseconds())
	if timeoutMs <= 0 {
		timeoutMs = 10000
	}
	var cerr *C.char
	rc := C.nrpe_ssl_connect(chost, cport, timeoutMs, &cconn, &cerr)
	if rc != 0 {
		defer freeCStr(cerr)
		return nil, errors.New(goString(cerr))
	}
	return &Conn{
		c:          cconn,
		remoteAddr: staticAddr{network: "nrpe+tls", addr: addr},
		localAddr:  staticAddr{network: "nrpe+tls", addr: ""},
	}, nil
}

func (c *Conn) Read(b []byte) (int, error) {
	if c.closed {
		return 0, net.ErrClosed
	}
	if len(b) == 0 {
		return 0, nil
	}
	var cerr *C.char
	n := C.nrpe_ssl_read(c.c, unsafe.Pointer(&b[0]), C.size_t(len(b)), &cerr)
	if n >= 0 {
		if n == 0 {
			return 0, io.EOF
		}
		return int(n), nil
	}
	return 0, translateError(cerr)
}

func (c *Conn) Write(b []byte) (int, error) {
	if c.closed {
		return 0, net.ErrClosed
	}
	if len(b) == 0 {
		return 0, nil
	}
	var cerr *C.char
	n := C.nrpe_ssl_write(c.c, unsafe.Pointer(&b[0]), C.size_t(len(b)), &cerr)
	if n >= 0 {
		return int(n), nil
	}
	return 0, translateError(cerr)
}

func (c *Conn) Close() error {
	if c.closed {
		return nil
	}
	C.nrpe_ssl_close(c.c)
	c.closed = true
	return nil
}

func (c *Conn) LocalAddr() net.Addr  { return c.localAddr }
func (c *Conn) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *Conn) SetDeadline(t time.Time) error {
	if c.closed {
		return net.ErrClosed
	}
	var timeout time.Duration
	if t.IsZero() {
		timeout = 0
	} else {
		timeout = time.Until(t)
		if timeout <= 0 {
			timeout = time.Millisecond
		}
	}
	timeoutMs := C.long(timeout.Milliseconds())
	var cerr *C.char
	rc := C.nrpe_ssl_set_timeout(c.c, timeoutMs, &cerr)
	if rc != 0 {
		return translateError(cerr)
	}
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error  { return c.SetDeadline(t) }
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.SetDeadline(t) }

func freeCStr(ptr *C.char) {
	if ptr != nil {
		C.free(unsafe.Pointer(ptr))
	}
}

func goString(ptr *C.char) string {
	if ptr == nil {
		return "unknown error"
	}
	return C.GoString(ptr)
}

func translateError(cstr *C.char) error {
	defer freeCStr(cstr)
	msg := goString(cstr)
	if strings.Contains(msg, "timed out") {
		return timeoutError(msg)
	}
	return errors.New(msg)
}

type timeoutError string

func (e timeoutError) Error() string { return string(e) }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func (c *Conn) String() string {
	return fmt.Sprintf("nrpe tls conn %s", c.remoteAddr)
}
