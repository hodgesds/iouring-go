// +build linux

package iouring

import (
	"syscall"

	"github.com/pkg/errors"
)

// PrepareClose is used to prep a nop.
func (r *Ring) PrepareClose(fd int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}
	sqe.Opcode = Close
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)

	ready()
	return sqe.UserData, nil
}

// Close is a nop.
func (r *Ring) Close(fd int) error {
	id, err := r.PrepareClose(fd)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}
