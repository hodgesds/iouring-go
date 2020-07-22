// +build linux

package iouring

import (
	"syscall"

	"github.com/pkg/errors"
)

// Splice implements splice using a ring.
func (r *Ring) Splice(
	inFd int,
	inOff int64,
	outFd int,
	outOff int64,
	n int,
	flags int,
) error {
	id, err := r.prepareSplice(inFd, inOff, outFd, outOff, n, flags)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

func (r *Ring) prepareSplice(
	inFd int,
	inOff int64,
	outFd int,
	outOff int64,
	n int,
	flags int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = Splice
	sqe.Fd = int32(outFd)
	sqe.Addr = uint64(inOff)
	sqe.Len = uint32(n)
	sqe.Offset = uint64(outOff)
	sqe.UFlags = int32(flags)
	// BUG: need to convert the inFd to the union member of the SQE
	//sqe.Anon0 = []byte(inFd)

	reqID := r.ID()
	sqe.UserData = reqID

	ready()
	return reqID, nil
}
