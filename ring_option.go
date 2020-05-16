// +build linux

package iouring

import (
	"golang.org/x/sys/unix"
)

// RingOption is an option for configuring a Ring.
type RingOption func(*Ring) error

// WithFileRegistry is used to configure a FileRegistry for use with a Ring.
func WithFileRegistry(reg FileRegistry) RingOption {
	return func(r *Ring) error {
		r.fileReg = reg
		return nil
	}
}

// WithDebug is used to print additional debug information.
func WithDebug() RingOption {
	return func(r *Ring) error {
		r.debug = true
		return nil
	}
}

// WithEventFd is used to add an eventfd to the Ring.
func WithEventFd(initval uint, flags int) RingOption {
	return func(r *Ring) error {
		fd, err := unix.Eventfd(initval, flags)
		if err != nil {
			return err
		}
		r.eventFd = fd
		return nil
	}
}
