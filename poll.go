// +build linux

package iouring

import (
	"runtime"
	"syscall"

	"github.com/pkg/errors"
)

// PollAdd is used to add a poll to a fd.
func (r *Ring) PollAdd(fd int, mask int) error {
	id, err := r.PreparePollAdd(fd, mask)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	runtime.KeepAlive(fd)
	return nil
}

// PreparePollAdd is used to prepare a SQE for adding a poll.
func (r *Ring) PreparePollAdd(fd int, mask int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}
	sqe.Opcode = PollAdd
	sqe.Fd = int32(fd)
	sqe.UFlags = int32(mask)
	sqe.UserData = r.ID()

	ready()
	return sqe.UserData, nil
}
