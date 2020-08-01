// +build linux

package iouring

import (
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// PrepareReadv is used to prepare a readv SQE.
func (r *Ring) PrepareReadv(
	fd int,
	iovecs []*syscall.Iovec,
	offset int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = Readv
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&iovecs[0])))
	sqe.Len = uint32(len(iovecs))
	sqe.Offset = uint64(offset)

	ready()
	return sqe.UserData, nil
}
