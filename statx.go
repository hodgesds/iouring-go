// +build linux

package iouring

import (
	"reflect"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Statx implements statx using a ring.
func (r *Ring) Statx(
	dirfd int,
	path string,
	flags int,
	mask int,
	statx *unix.Statx_t,
) (err error) {
	id, err := r.prepareStatx(dirfd, path, flags, mask, statx)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	// No GC until the request is done.
	runtime.KeepAlive(statx)
	runtime.KeepAlive(dirfd)
	runtime.KeepAlive(path)
	runtime.KeepAlive(mask)
	runtime.KeepAlive(flags)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

func (r *Ring) prepareStatx(
	dirfd int,
	path string,
	flags int,
	mask int,
	statx *unix.Statx_t,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = Statx
	sqe.Fd = int32(dirfd)
	if path != "" {
		// TODO: could probably avoid the conversion to []byte
		b := saferStringToBytes(&path)
		sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	}
	sqe.Len = uint32(mask)
	sqe.Offset = (uint64)(uintptr(unsafe.Pointer(statx)))
	sqe.UFlags = int32(flags)

	reqID := r.ID()
	sqe.UserData = reqID

	ready()
	return reqID, nil
}

func saferStringToBytes(s *string) []byte {
	bytes := make([]byte, 0, 0)

	// Shameless stolen from:
	// See: https://github.com/jlauinger/go-safer
	// create the string and slice headers by casting. Obtain pointers to the
	// headers to be able to change the slice header properties in the next step
	stringHeader := (*reflect.StringHeader)(unsafe.Pointer(s))
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&bytes))

	// set the slice's length and capacity temporarily to zero (this is actually
	// unnecessary here because the slice is already initialized as zero, but if
	// you are reusing a different slice this is important
	sliceHeader.Len = 0
	sliceHeader.Cap = 0

	// change the slice header data address
	sliceHeader.Data = stringHeader.Data

	// set the slice capacity and length to the string length
	sliceHeader.Cap = stringHeader.Len
	sliceHeader.Len = stringHeader.Len

	// use the keep alive dummy function to make sure the original string s is not
	// freed up until this point
	runtime.KeepAlive(s)

	return bytes
}
