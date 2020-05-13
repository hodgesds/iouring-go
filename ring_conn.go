// +build linux

package iouring

import (
	"net"
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/errors"
)

// ringConn is a net.Conn that is backed by the Ring.
type ringConn struct {
	fd     int
	laddr  *addr
	raddr  *addr
	r      *Ring
	offset *int64
	stop   chan struct{}
	poll   chan uint64
}

// getCqe is used for getting a CQE result.
func (c *ringConn) getCqe(reqID uint64) (int, error) {
	err := c.r.Enter(uint(1024), uint(1), EnterGetEvents, nil)
	if err != nil {
		return 0, err
	}
	cqe, err := c.r.cq.EntryBy(reqID)
	if err != nil {
		return 0, err
	}
	if cqe.Res < 0 {
		return int(cqe.Res), syscall.Errno(cqe.Res)
	}

	// Reenable the poll on the connection.
	id := c.r.ID()
	sqe, commit := c.r.SubmitEntry()
	sqe.Opcode = PollAdd
	sqe.Fd = int32(c.fd)
	sqe.UFlags = int32(pollin)
	sqe.UserData = id
	commit()
	c.poll <- id

	return int(cqe.Res), nil
}

func (c *ringConn) run() {
	for {
		select {
		case <-c.stop:
			id := c.r.ID()
			sqe, commit := c.r.SubmitEntry()
			sqe.Opcode = PollRemove
			sqe.Fd = int32(c.fd)
			sqe.UFlags = int32(pollin)
			sqe.UserData = id
			commit()
			c.getCqe(id)
			return
		case id := <-c.poll:
			c.getCqe(id)
		}
	}
}

// Read implements the net.Conn interface.
func (c *ringConn) Read(b []byte) (n int, err error) {
	sqe, ready := c.r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = ReadFixed
	sqe.Fd = int32(c.fd)
	sqe.Len = uint32(len(b))
	sqe.Flags = 0
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	reqID := c.r.ID()
	sqe.UserData = reqID

	ready()

	return c.getCqe(reqID)
}

// Write implements the net.Conn interface.
func (c *ringConn) Write(b []byte) (n int, err error) {
	sqe, ready := c.r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = WriteFixed
	sqe.Fd = int32(c.fd)
	sqe.Len = uint32(len(b))
	sqe.Flags = 0
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	reqID := c.r.ID()
	sqe.UserData = reqID

	ready()

	return c.getCqe(reqID)
}

// Close implements the net.Conn interface.
func (c *ringConn) Close() error {
	c.stop <- struct{}{}
	return syscall.Close(c.fd)
}

// LocalAddr implements the net.Conn interface.
func (c *ringConn) LocalAddr() net.Addr {
	return c.laddr
}

// RemoteAddr implements the net.Conn interface.
func (c *ringConn) RemoteAddr() net.Addr {
	return c.raddr
}

// SetDeadline implements the net.Conn interface.
func (c *ringConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements the net.Conn interface.
func (c *ringConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline the net.Conn interface.
func (c *ringConn) SetWriteDeadline(t time.Time) error {
	return nil
}
