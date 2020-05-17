// +build linux

package iouring

import (
	"golang.org/x/sys/unix"
)

// RingOption is an option for configuring a Ring.
type RingOption func(*Ring) error

// WithDebug is used to print additional debug information.
func WithDebug() RingOption {
	return func(r *Ring) error {
		r.debug = true
		return nil
	}
}

// WithEventFd is used to create an eventfd and register it to the Ring.
// The event fd can be accessed using the EventFd method.
func WithEventFd(initval uint, flags int, async bool) RingOption {
	return func(r *Ring) error {
		fd, err := unix.Eventfd(initval, flags)
		if err != nil {
			return err
		}
		r.eventFd = fd
		if async {
			return RegisterEventFdAsync(r.fd, fd)
		}
		return RegisterEventFd(r.fd, fd)
	}
}

// WithFileRegistry is used to register a FileRegistry with the Ring. The
// registery can be accessed with the FileRegistry method on the ring.
func WithFileRegistry() RingOption {
	return func(r *Ring) error {
		r.fileReg = NewFileRegistry(r.fd)
		return nil
	}
}
