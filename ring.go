package iouring

import (
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Ring contains an io_uring submit and completion ring.
type Ring struct {
	fd   int
	p    *Params
	cq   *CompletionQueue
	cqMu sync.RWMutex
	sq   *SubmitQueue
	sqMu sync.RWMutex
}

// New is used to create an iouring.Ring.
func New(size uint) (*Ring, error) {
	p := Params{}
	fd, err := Setup(size, &p)
	if err != nil {
		return nil, err
	}
	var (
		cq CompletionQueue
		sq SubmitQueue
	)
	if err := MmapRing(fd, &p, &sq, &cq); err != nil {
		return nil, err
	}
	return &Ring{
		p:  &p,
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
	if r.p.Flags&FeatSingleMmap != 0 {
		if err := r.closeCq(); err != nil {
			return err
		}
	}
	return syscall.Close(r.fd)
}

func (r *Ring) closeCq() error {
	r.cqMu.Lock()
	defer r.cqMu.Unlock()
	if r.cq == nil {
		return nil
	}

	_, _, errno := syscall.Syscall6(
		syscall.SYS_MUNMAP,
		r.cq.ptr,
		uintptr(r.cq.Size),
		uintptr(0),
		uintptr(0),
		uintptr(0),
		uintptr(0),
	)
	if errno != 0 {
		err := errno
		return errors.Wrap(err, "failed to munmap cq ring")
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

	_, _, errno := syscall.Syscall6(
		syscall.SYS_MUNMAP,
		r.sq.ptr,
		uintptr(r.sq.Size),
		uintptr(0),
		uintptr(0),
		uintptr(0),
		uintptr(0),
	)
	if errno != 0 {
		err := errno
		return errors.Wrap(err, "failed to munmap sq ring")
	}
	r.sq = nil
	return nil
}

// SubmitHead returns the position of the head of the submit queue. This method
// is safe for calling concurrently.
func (r *Ring) SubmitHead() int {
	r.sqMu.RLock()
	defer r.sqMu.RUnlock()
	if r.sq == nil || r.sq.Head == nil {
		return 0
	}
	return int(*r.sq.Head)
}

// SubmitTail returns the position of the tail of the submit queue. This method
// is safe for calling concurrently.
func (r *Ring) SubmitTail() int {
	r.sqMu.RLock()
	defer r.sqMu.RUnlock()
	if r.sq == nil || r.sq.Tail == nil {
		return 0
	}
	return int(*r.sq.Tail)
}
