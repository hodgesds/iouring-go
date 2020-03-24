// +build linux

package iouring

import (
	"syscall"
	"unsafe"
)

// RegisterFiles is used to register files to a ring.
func RegisterFiles(fd int, files []int) error {
	_, _, errno := syscall.Syscall6(
		RegisterSyscall,
		uintptr(fd),
		uintptr(RegRegisterFiles),
		uintptr(unsafe.Pointer(&files[0])),
		uintptr(len(files)),
		uintptr(0),
		uintptr(0),
	)
	if errno < 0 {
		var err error
		err = errno
		return err
	}
	return nil
}
