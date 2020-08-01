// +build linux

package iouring

import (
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// PrepareConnect is used to prepare a SQE for a connect(2) call.
func (r *Ring) PrepareConnect(fd int, addr syscall.Sockaddr, socklen uint32) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = Connect
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&addr)))
	sqe.Len = socklen

	ready()
	return sqe.UserData, nil
}
