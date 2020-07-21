// +build linux

package iouring

import (
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

	// BUG: `path` string pointer doesn't seem to work properly.
	// See liburing io_uring_prep_statx:
	// https://github.com/axboe/liburing/blob/master/src/include/liburing.h#L371
	// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/fs/io_uring.c?h=v5.8-rc6#n3385
	sqe.Opcode = Statx
	sqe.Fd = int32(dirfd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&path)))
	sqe.Len = uint32(mask)
	sqe.Offset = (uint64)(uintptr(unsafe.Pointer(statx)))
	sqe.UFlags = int32(flags)

	reqID := r.ID()
	sqe.UserData = reqID

	ready()
	return reqID, nil
}
