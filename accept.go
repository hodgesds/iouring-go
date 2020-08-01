// +build linux

package iouring

import (
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// PrepareAccept is used to prepare a SQE for an accept(2) call.
func (r *Ring) PrepareAccept(
	fd int,
	addr syscall.Sockaddr,
	socklen uint32,
	flags int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = Accept
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&addr)))
	sqe.Offset = uint64(socklen)
	sqe.UFlags = int32(flags)

	ready()
	return sqe.UserData, nil
}
