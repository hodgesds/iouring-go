// +build linux

package iouring

import (
	"reflect"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

var (
	errInvalidEntries = errors.New("entries must be a power of 2 from 1 to 4096, inclusive")
	uint32Size        = unsafe.Sizeof(uint32(0))
	cqeSize           = unsafe.Sizeof(CompletionEntry{})
	seSize            = unsafe.Sizeof(SubmitEntry{})
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
		if errno != 0 {
			err = errno
			return errors.Wrap(err, "failed to mmap cq ring")
		}
	}

	// Conversion of a uintptr back to Pointer is not valid in general,
	// except for:
	// 3) Conversion of a Pointer to a uintptr and back, with arithmetic.
	sq.Head = (*uint32)(unsafe.Pointer(sqPtr + uintptr(p.SqOffset.Head)))
	sq.Tail = (*uint32)(unsafe.Pointer(uintptr(uint(sqPtr) + uint(p.SqOffset.Tail))))
	sq.Mask = (*uint32)(unsafe.Pointer(uintptr(uint(sqPtr) + uint(p.SqOffset.RingMask))))
	sq.Flags = (*uint32)(unsafe.Pointer(uintptr(uint(sqPtr) + uint(p.SqOffset.Flags))))
	sq.Dropped = (*uint32)(unsafe.Pointer(uintptr(uint(sqPtr) + uint(p.SqOffset.Dropped))))

	// Making mmap'd slices is annoying.
	sq.Entries = *(*[]SubmitEntry)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(uint(sqPtr) + uint(p.SqOffset.RingEntries)),
		Len:  int(p.SqEntries),
		Cap:  int(p.SqEntries),
	}))

	cq.Head = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.Head))))
	cq.Tail = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.Tail))))
	cq.Mask = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.RingMask))))
	cq.Overflow = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.CqOffset.Overflow))))

	cq.Entries = *(*[]CompletionEntry)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(uint(cqPtr) + uint(p.CqOffset.RingEntries)),
		Len:  int(p.CqEntries),
		Cap:  int(p.CqEntries),
	}))

	cq.ptr = cqPtr
	sq.ptr = sqPtr
	return nil
}
