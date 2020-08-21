// +build linux

package iouring

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// RegisterEventFd is used to register an event file descriptor to a ring.
func RegisterEventFd(ringFd int, fd int) error {
	_, _, errno := syscall.Syscall6(
		RegisterSyscall,
		uintptr(ringFd),
		uintptr(RegRegisterEventFd),
		uintptr(fd),
		uintptr(1),
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

// RegisterEventFdAsync is used to register an event file descriptor for async
// polling on a ring.
func RegisterEventFdAsync(ringFd int, fd int) error {
	_, _, errno := syscall.Syscall6(
		RegisterSyscall,
		uintptr(ringFd),
		uintptr(RegRegisterEventFdAsync),
		uintptr(fd),
		uintptr(1),
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

// UnregisterEventFd is used to unregister a file descriptor to a ring.
func UnregisterEventFd(ringFd int, fd int) error {
	_, _, errno := syscall.Syscall6(
		RegisterSyscall,
		uintptr(ringFd),
		uintptr(RegRegisterEventFd),
		uintptr(0),
		uintptr(0),
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

// RegisterBuffers is used to register buffers to a ring.
func RegisterBuffers(fd int, vecs []*syscall.Iovec) error {
	_, _, errno := syscall.Syscall6(
		RegisterSyscall,
		uintptr(fd),
		uintptr(RegRegisterBuffers),
		uintptr(unsafe.Pointer(&vecs[0])),
		uintptr(len(vecs)),
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

// UnregisterBuffers is used to unregister iovecs from a ring.
func UnregisterBuffers(fd int, vecs []*syscall.Iovec) error {
	_, _, errno := syscall.Syscall6(
		RegisterSyscall,
		uintptr(fd),
		uintptr(RegUnregisterBuffers),
		uintptr(unsafe.Pointer(&vecs[0])),
		uintptr(len(vecs)),
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

// FileRegistry is an interface for registering files to a Ring.
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

// NewFileRegistry creates a FileRegistry for use with a ring.
func NewFileRegistry(ringFd int) FileRegistry {
	return &fileRegistry{
		ringFd: ringFd,
		f:      []int{},
		fID:    map[int]int{},
	}
}

// Register implements the FileRegistry interface. It is used to register a
// file descriptor with a ring.
func (r *fileRegistry) Register(fd int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.f = append(r.f, fd)
	r.fID[fd] = len(r.f) - 1
	if r.fID[fd] < 0 {
		r.fID[fd] = 0
	}
	return ReregisterFiles(r.ringFd, r.f)
}

// Unregister implements the FileRegistry interface. It is used to unregister a
// file descriptor form a ring.
func (r *fileRegistry) Unregister(fd int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, ok := r.fID[fd]
	if !ok {
		return fmt.Errorf("fd %d not registered", fd)
	}
	r.f = append(r.f[:id], r.f[id+1:]...)

	return UnregisterFiles(r.ringFd, r.f)
}

// ID returns the ID of a file descriptor that has been registered.
func (r *fileRegistry) ID(fd int) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.fID[fd]
	return id, ok
}
