// +build linux

package iouring

import (
	"errors"
	"reflect"
	"syscall"
	"unsafe"
)

var (
	errInvalidEntries = errors.New("entries must be a power of 2 from 1 to 4096, inclusive")
	uint32Size        = unsafe.Sizeof(uint(0))
	cqeSize           = unsafe.Sizeof(CompletionEntry{})
	seSize            = unsafe.Sizeof(SubmitEntry{})
)

// Setup is used to setup a io_uring using the io_uring_setup syscall.
func Setup(entries uint, params *Params) (int, error) {
	if entries < 1 || entries > 4096 || !((entries & (entries - 1)) == 0) {
		return 0, errInvalidEntries
	}
	_, _, errno := syscall.Syscall(SetupSyscall, uintptr(entries), uintptr(unsafe.Pointer(params)), uintptr(0))
	if errno < 0 {
		var err error
		err = errno
		return 0, err
	}
	return int(errno), nil
}

// MmapSubmitRing is used to configured the submit queue, it should only be
// called after the Setup function has completed successfully.
// See:
// https://github.com/axboe/liburing/blob/master/src/setup.c#L22
func MmapSubmitRing(fd int, p *Params, sq *SubmitQueue) error {
	sq.Size = uint(p.SqOffset.Array) + (uint(p.SqEntries) * uint(uint32Size))
	ptr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		uintptr(0),
		uintptr(sq.Size),
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(0),
		uintptr(fd),
		uintptr(SqRingOffset),
	)
	if errno < 0 {
		var err error
		err = errno
		return err
	}

	// Conversion of a uintptr back to Pointer is not valid in general,
	// except for:
	// 3) Conversion of a Pointer to a uintptr and back, with arithmetic.
	sq.Head = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.SqOffset.Head))))
	sq.Tail = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.SqOffset.Tail))))
	sq.Mask = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.SqOffset.RingMask))))
	sq.Flags = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.SqOffset.Flags))))
	sq.Dropped = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.SqOffset.Dropped))))

	// Making mmap'd slices is annoying.
	sq.Entries = *(*[]SubmitEntry)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(uint(ptr) + uint(p.SqOffset.RingEntries)),
		Len:  int(p.SqEntries),
		Cap:  int(p.SqEntries),
	}))

	return nil
}

// MmapCompletionRing is used to mmap the completion ring buffer.
func MmapCompletionRing(fd int, p *Params, cq *CompletionQueue) error {
	cq.Size = uint(p.CqOffset.Cqes) + (uint(p.CqEntries) * uint(cqeSize))
	ptr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		uintptr(0),
		uintptr(cq.Size),
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(0),
		uintptr(fd),
		uintptr(CqRingOffset),
	)
	if errno < 0 {
		var err error
		err = errno
		return err
	}

	cq.Head = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.CqOffset.Head))))
	cq.Tail = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.CqOffset.Tail))))
	cq.Mask = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.CqOffset.RingMask))))
	cq.Overflow = (*uint)(unsafe.Pointer(uintptr(uint(ptr) + uint(p.CqOffset.Overflow))))

	cq.Entries = *(*[]CompletionEntry)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(uint(ptr) + uint(p.CqOffset.RingEntries)),
		Len:  int(p.CqEntries),
		Cap:  int(p.CqEntries),
	}))

	return nil
}
