// +build linux

package iouring

import (
	"context"
	"net"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/errors"
)

// ringConn is a net.Conn that is backed by the Ring.
type ringConn struct {
	fd        int
	laddr     *addr
	raddr     *addr
	r         *Ring
	offset    *int64
	stop      chan struct{}
	poll      chan uint64
	pollReady *int32

	deadMu        sync.RWMutex
	deadline      time.Time
	readDeadline  time.Time
	writeDeadline time.Time
}

// getCqe is used for getting a CQE result.
func (c *ringConn) getCqe(ctx context.Context, reqID uint64) (int, error) {
	// TODO: Where should this repoll go?
	_, err := c.r.Enter(uint(1024), uint(1), EnterGetEvents, nil)
	if err != nil {
		return 0, err
	}
	c.stop <- struct{}{}
	var cqe *CompletionEntry
	for {
		select {
		case <-ctx.Done():
			return 0, syscall.ETIMEDOUT
		default:
		}
		cqe, err = c.r.cq.EntryBy(reqID)
		if err != nil {
			// TODO: How many tries should looking for the cqe be
			// tried?
			if err != ErrEntryNotFound {
				continue
			}
			return 0, err
		}
		break
	}
	res := int(cqe.Res)
	if res < 0 {
		return 0, syscall.Errno(-res)
	}

	return res, nil
}

func (c *ringConn) rePoll() {
	// Reenable the poll on the connection.
	id := c.r.ID()
	sqe, commit := c.r.SubmitEntry()
	sqe.Opcode = PollAdd
	sqe.Fd = int32(c.fd)
	sqe.UFlags = int32(POLLIN)
	sqe.UserData = id
	commit()
	c.r.Enter(uint(1024), uint(1), EnterGetEvents, nil)
}

func (c *ringConn) run() {
	for {
		select {
		case <-c.stop:
			id := c.r.ID()
			sqe, commit := c.r.SubmitEntry()
			sqe.Opcode = PollRemove
			sqe.Fd = int32(c.fd)
			sqe.UserData = id
			commit()
			c.getCqe(context.Background(), id)
			return
		}
	}
}

// Read implements the net.Conn interface.
func (c *ringConn) Read(b []byte) (int, error) {
	c.rePoll()
	sqe, commit := c.r.SubmitEntry()
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
	commit()
	ctx := context.Background()

	n, err := c.getCqe(ctx, reqID)
	runtime.KeepAlive(b)
	return n, err
}

// Write implements the net.Conn interface.
func (c *ringConn) Write(b []byte) (n int, err error) {
	sqe, commit := c.r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = WriteFixed
	sqe.Fd = int32(c.fd)
	sqe.Len = uint32(len(b))
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	reqID := c.r.ID()
	sqe.UserData = reqID
	commit()

	n, err = c.getCqe(context.Background(), reqID)
	runtime.KeepAlive(b)
	return n, err
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
	c.deadMu.Lock()
	c.deadline = t
	c.deadMu.Unlock()
	return nil
}

// SetReadDeadline implements the net.Conn interface.
func (c *ringConn) SetReadDeadline(t time.Time) error {
	c.deadMu.Lock()
	c.readDeadline = t
	c.deadMu.Unlock()
	return nil
}

// SetWriteDeadline the net.Conn interface.
func (c *ringConn) SetWriteDeadline(t time.Time) error {
	c.deadMu.Lock()
	c.writeDeadline = t
	c.deadMu.Unlock()
	return nil
}
