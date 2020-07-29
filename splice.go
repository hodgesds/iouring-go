// +build linux

package iouring

import (
	"encoding/binary"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// Splice implements splice using a ring.
func (r *Ring) Splice(
	inFd int,
	inOff *int64,
	outFd int,
	outOff *int64,
	n int,
	flags int,
) (int64, error) {
	id, err := r.PrepareSplice(inFd, inOff, outFd, outOff, n, flags)
	if err != nil {
		return 0, err
	}
	// TODO: replace complete with something more efficient.
	errno, res := r.complete(id)
	if errno < 0 {
		return 0, syscall.Errno(-errno)
	}
	runtime.KeepAlive(inOff)
	runtime.KeepAlive(outOff)
	return int64(res), nil
}

// PrepareSplice is used to prepare a SQE for a splice(2).
func (r *Ring) PrepareSplice(
	inFd int,
	inOff *int64,
	outFd int,
	outOff *int64,
	n int,
	flags int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = Splice
	sqe.Fd = int32(outFd)
	if inOff != nil {
		sqe.Addr = uint64(uintptr(unsafe.Pointer(&inOff)))
	} else {
		sqe.Addr = 0
	}
	sqe.Len = uint32(n)
	if outOff != nil {
		sqe.Offset = uint64(uintptr(unsafe.Pointer(&outOff)))
	} else {
		sqe.Offset = 0
	}
	sqe.UFlags = int32(flags)
	// BUG: need to convert the inFd to the union member of the SQE
	anon := [24]byte{}
	binary.LittleEndian.PutUint32(anon[4:], uint32(inFd))
	sqe.Anon0 = anon
	sqe.UserData = r.ID()

	ready()
	return sqe.UserData, nil
}
