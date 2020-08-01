// +build linux

package iouring

import (
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// PrepareTimeout is used to prepare a timeout SQE.
func (r *Ring) PrepareTimeout(ts *unix.Timespec, count int, flags int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = Timeout
	sqe.UserData = r.ID()
	sqe.UFlags = int32(flags)
	sqe.Fd = -1
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(ts)))
	sqe.Len = 1
	sqe.Offset = uint64(count)

	ready()
	return sqe.UserData, nil
}

// PrepareTimeoutRemove is used to prepare a timeout removal.
func (r *Ring) PrepareTimeoutRemove(data uint64, flags int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = TimeoutRemove
	sqe.UserData = r.ID()
	sqe.UFlags = int32(flags)
	sqe.Fd = -1
	sqe.Addr = data
	sqe.Len = 0
	sqe.Offset = 0

	ready()
	return sqe.UserData, nil
}
