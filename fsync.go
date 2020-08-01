// +build linux

package iouring

import (
	"syscall"

	"github.com/pkg/errors"
)

// PrepareFsync is used to prep a nop.
func (r *Ring) PrepareFsync(fd int, flags int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}
	sqe.Opcode = Fsync
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.UFlags = int32(flags)

	ready()
	return sqe.UserData, nil
}

// Fsync is a nop.
func (r *Ring) Fsync(fd int, flags int) error {
	id, err := r.PrepareFsync(fd, flags)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}
