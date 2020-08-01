// +build linux

package iouring

import (
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// PrepareWritev is used to prepare a writev SQE.
func (r *Ring) PrepareWritev(
	fd int,
	iovecs []*syscall.Iovec,
	offset int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = Writev
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&iovecs[0])))
	sqe.Len = uint32(len(iovecs))
	sqe.Offset = uint64(offset)

	ready()
	return sqe.UserData, nil
}
