// +build linux

package iouring

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Enter is used to submit to the queue.
func Enter(fd int, toSubmit uint, minComplete uint, flags uint, sigset *unix.Sigset_t) error {
	_, _, errno := syscall.Syscall6(
		EnterSyscall,
		uintptr(fd),
		uintptr(toSubmit),
		uintptr(minComplete),
		uintptr(flags),
		uintptr(unsafe.Pointer(sigset)),
		uintptr(0),
	)
	if errno < 0 {
		var err error
		err = errno
		return err
	}

	return nil
}
