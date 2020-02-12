package iouring

import "golang.org/x/sys/unix"

// Ring is an interface for interacting with the io_uring interface.
type Ring interface {
	Submit(toSubmit uint, minComplete uint, flags uint, sigset *unix.Sigset_t) error
}
