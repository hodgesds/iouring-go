// +build linux

package iouring

import (
	"syscall"

	"github.com/pkg/errors"
)

// PrepareNop is used to prep a nop.
func (r *Ring) PrepareNop() (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}
	sqe.Opcode = Nop
	sqe.UserData = r.ID()
	sqe.Fd = -1

	ready()
	return sqe.UserData, nil
}

// Nop is a nop.
func (r *Ring) Nop() error {
	id, err := r.PrepareNop()
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}
