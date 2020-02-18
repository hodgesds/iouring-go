package iouring

import (
	"reflect"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Ring contains an io_uring submit and completion ring.
type Ring struct {
	fd   int
	cq   *CompletionQueue
	cqMu sync.RWMutex
	sq   *SubmitQueue
	sqMu sync.RWMutex
}

// New is used to create an iouring.Ring.
func New(size uint, p *Params) (*Ring, error) {
	fd, err := Setup(size, p)
	if err != nil {
		return nil, err
	}
	var sq SubmitQueue
	if err := MmapSubmitRing(fd, p, &sq); err != nil {
		return nil, err
	}
	var cq CompletionQueue
	if err := MmapCompletionRing(fd, p, &cq); err != nil {
		return nil, err
	}

	return &Ring{
		fd: fd,
		cq: &cq,
		sq: &sq,
	}, nil
}

func (r *Ring) Submit(toSubmit uint, minComplete uint, flags uint, sigset *unix.Sigset_t) error {
	// XXX: Make these options.
	if err := Enter(r.fd, toSubmit, minComplete, EnterGetevents, nil); err != nil {
		return err
	}
	return nil
}

// Close is used to close the ring.
func (r *Ring) Close() error {
	if err := r.closeSq(); err != nil {
		return err
	}
	if err := r.closeCq(); err != nil {
		return err
	}
	return syscall.Close(r.fd)
}

func (r *Ring) closeCq() error {
	r.cqMu.Lock()
	defer r.cqMu.Unlock()
	if r.cq == nil {
		return nil
	}

	err := syscall.Munmap(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&r.cq.Entries[0])),
		Len:  len(r.cq.Entries) * int(cqeSize),
		Cap:  len(r.cq.Entries) * int(cqeSize),
	})))
	if err != nil {
		return err
	}

	r.cq = nil
	return nil
}

func (r *Ring) closeSq() error {
	r.sqMu.Lock()
	defer r.sqMu.Unlock()
	if r.sq == nil {
		return nil
	}

	err := syscall.Munmap(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&r.sq.Entries[0])),
		Len:  len(r.sq.Entries) * int(seSize),
		Cap:  len(r.sq.Entries) * int(seSize),
	})))
	if err != nil {
		return err
	}

	r.sq = nil
	return nil
}
