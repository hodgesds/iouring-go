// +build linux

package iouring

import (
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// PrepareRecvmsg is used to prepare a recvmsg SQE.
func (r *Ring) PrepareRecvmsg(
	fd int,
	msg *syscall.Msghdr,
	flags int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = RecvMsg
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(msg)))
	sqe.Len = 1
	sqe.Offset = 0
	sqe.UFlags = int32(flags)

	ready()
	return sqe.UserData, nil
}
