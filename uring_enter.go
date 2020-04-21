// +build linux

package iouring

import (
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Enter is used to submit to the queue.
func Enter(fd int, toSubmit uint, minComplete uint, flags uint, sigset *unix.Sigset_t) (int, error) {
	res, _, errno := syscall.Syscall6(
		EnterSyscall,
		uintptr(fd),
		uintptr(toSubmit),
		uintptr(minComplete),
		uintptr(flags),
		uintptr(unsafe.Pointer(sigset)),
		uintptr(0),
	)
	if errno != 0 {
		var err error
		err = errno
		return 0, err
	}
	if res < 0 {
		return 0, errors.New("no entries completed")
	}

	return int(res), nil
}
