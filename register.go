// +build linux

package iouring

import (
	"fmt"
	"sync"
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

// UnregisterFiles is used to unregister files to a ring.
func UnregisterFiles(fd int, files []int) error {
	_, _, errno := syscall.Syscall6(
		RegisterSyscall,
		uintptr(fd),
		uintptr(RegUnregisterFiles),
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

// ReregisterFiles is used to reregister files to a ring.
func ReregisterFiles(fd int, files []int) error {
	_, _, errno := syscall.Syscall6(
		RegisterSyscall,
		uintptr(fd),
		uintptr(RegRegisterFilesUpdate),
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

type FileRegistry interface {
	Register(int) error
	Unregister(int) error
	ID(int) (int, bool)
}

type fileRegistry struct {
	mu     sync.RWMutex
	ringFd int
	f      []int
	fID    map[int]int /* map of fd to offset */
}

func NewFileRegistry(ringFd int) FileRegistry {
	return &fileRegistry{
		ringFd: ringFd,
		f:      []int{},
		fID:    map[int]int{},
	}
}

func (r *fileRegistry) Register(fd int) error {
	r.mu.Lock()
	r.mu.RUnlock()

	r.f = append(r.f, fd)
	r.fID[fd] = len(r.f) - 1
	if r.fID[fd] < 0 {
		r.fID[fd] = 0
	}
	return RegisterFiles(r.ringFd, r.f)
}

func (r *fileRegistry) Unregister(fd int) error {
	r.mu.Lock()
	r.mu.RUnlock()

	id, ok := r.fID[fd]
	if !ok {
		return fmt.Errorf("fd %d not registered", fd)
	}
	r.f = append(r.f[:id], r.f[id+1:]...)

	return RegisterFiles(r.ringFd, r.f)
}

func (r *fileRegistry) ID(fd int) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.fID[fd]
	return id, ok
}
