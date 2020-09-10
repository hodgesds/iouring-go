// +build linux

package iouring

import (
	"reflect"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

var (
	uint32Size = unsafe.Sizeof(uint32(0))
	cqeSize    = unsafe.Sizeof(CompletionEntry{})
	sqeSize    = unsafe.Sizeof(SubmitEntry{})
)

// Setup is used to setup a io_uring using the io_uring_setup syscall.
func Setup(entries uint, params *Params) (int, error) {
	fd, _, errno := syscall.Syscall(
		SetupSyscall,
		uintptr(entries),
		uintptr(unsafe.Pointer(params)),
		uintptr(0),
	)
	if errno != 0 {
		err := errno
		return 0, err
	}
	return int(fd), nil
}

// MmapRing is used to configure the submit and completion queues, it should only
// be called after the Setup function has completed successfully.
// See:
// https://github.com/axboe/liburing/blob/master/src/setup.c#L22
func MmapRing(fd int, p *Params, sq *SubmitQueue, cq *CompletionQueue) error {
	var (
		cqPtr uintptr
		sqPtr uintptr
		errno syscall.Errno
		err   error
	)
	singleMmap := p.Flags&FeatSingleMmap != 0
	sq.Size = uint32(uint(p.SqOffset.Array) + (uint(p.SqEntries) * uint(uint32Size)))
	cq.Size = uint32(uint(p.CqOffset.Cqes) + (uint(p.CqEntries) * uint(cqeSize)))

	if singleMmap {
		if cq.Size > sq.Size {
			sq.Size = cq.Size
		} else {
			cq.Size = sq.Size
		}
	}

	sqPtr, _, errno = syscall.Syscall6(
		syscall.SYS_MMAP,
		uintptr(0),
		uintptr(sq.Size),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(fd),
		uintptr(SqRingOffset),
	)
	if errno != 0 {
		err = errno
		return errors.Wrap(err, "failed to mmap sq ring")
	}
	sq.ptr = sqPtr

	// Conversion of a uintptr back to Pointer is not valid in general,
	// except for:
	// 3) Conversion of a Pointer to a uintptr and back, with arithmetic.

	// Go vet doesn't like this so it's probably not valid.
	sq.Head = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(p.SqOffset.Head)))
	sq.Tail = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(p.SqOffset.Tail)))
	sq.Mask = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(p.SqOffset.RingMask)))
	sq.Flags = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(p.SqOffset.Flags)))
	sq.Dropped = (*uint32)(unsafe.Pointer(sq.ptr + uintptr(p.SqOffset.Dropped)))

	// Map the sqe ring.
	sqePtr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		uintptr(0),
		uintptr(uint(p.SqEntries)*uint(sqeSize)),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(fd),
		uintptr(SqeRingOffset),
	)
	if errno < 0 {
		return syscall.Errno(-errno)
	}

	// Making mmap'd slices is annoying.
	// BUG: don't use composite literals
	sq.Entries = *(*[]SubmitEntry)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(sqePtr),
		Len:  int(p.SqEntries),
		Cap:  int(p.SqEntries),
	}))
	// BUG: don't use composite literals
	sq.Array = *(*[]uint32)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(sqPtr + uintptr(p.SqOffset.Array))),
		Len:  int(p.SqEntries),
		Cap:  int(p.SqEntries),
	}))
	runtime.KeepAlive(sqePtr)

	if singleMmap {
		cqPtr = sqPtr
	} else {
		cqPtr, _, errno = syscall.Syscall6(
			syscall.SYS_MMAP,
			uintptr(0),
			uintptr(cq.Size),
			syscall.PROT_READ|syscall.PROT_WRITE,
			syscall.MAP_SHARED|syscall.MAP_POPULATE,
			uintptr(fd),
			uintptr(CqRingOffset),
		)
		if errno < 0 {
			return syscall.Errno(-errno)
		}
	}

	cq.Head = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.Head))))
	cq.Tail = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.Tail))))
	cq.Mask = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.RingMask))))
	cq.Overflow = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.Overflow))))
	cq.Flags = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.Flags))))

	// BUG: don't use composite literals
	cq.Entries = *(*[]CompletionEntry)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(uint(cqPtr) + uint(p.CqOffset.Cqes)),
		Len:  int(p.CqEntries),
		Cap:  int(p.CqEntries),
	}))
	// See: https://github.com/jlauinger/go-safer
	runtime.KeepAlive(cqPtr)

	return nil
}
