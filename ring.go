// +build linux

package iouring

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Ring contains an io_uring submit and completion ring.
type Ring struct {
	fd      int
	p       *Params
	cq      *CompletionQueue
	cqMu    sync.RWMutex
	sq      *SubmitQueue
	sqMu    sync.RWMutex
	sqPool  sync.Pool
	idx     *uint64
	debug   bool
	fileReg FileRegistry

	eventFd int
}

// New is used to create an iouring.Ring.
func New(size uint, p *Params, opts ...RingOption) (*Ring, error) {
	if p == nil {
		p = &Params{}
	}
	fd, err := Setup(size, p)
	if err != nil {
		return nil, err
	}
	var (
		cq       CompletionQueue
		sq       SubmitQueue
		sqWrites uint32
	)
	if err := MmapRing(fd, p, &sq, &cq); err != nil {
		return nil, err
	}
	idx := uint64(0)
	entered := uint32(0)
	sq.entered = &entered

	sq.writes = &sqWrites
	r := &Ring{
		p:       p,
		fd:      fd,
		cq:      &cq,
		sq:      &sq,
		idx:     &idx,
		fileReg: NewFileRegistry(fd),
		eventFd: -1,
	}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// EventFd returns the file descriptor of the eventfd if it is set, otherwise
// it returns the default value of -1.
func (r *Ring) EventFd() int {
	return r.eventFd
}

// Enter is used to enter the ring.
func (r *Ring) Enter(toSubmit uint, minComplete uint, flags uint, sigset *unix.Sigset_t) error {
	// Acquire the submit barrier so that the ring can safely be entered.
	if r.sq.NeedWakeup() {
		flags |= EnterSqWakeup
	}
	// Increase the write counter as the caller will be
	// updating the returned SubmitEntry.
	r.sq.enterLock()
	// TODO: Document how sigset should be used in relation with the go runtime and
	// io_uring_enter.
	completed, err := Enter(r.fd, toSubmit, minComplete, flags, sigset)
	r.sq.enterUnlock()
	if err != nil {
		return err
	}
	if completed < 0 {
		return fmt.Errorf("%d", completed)
	}
	return nil
}

func (r *Ring) canEnter() bool {
	return atomic.LoadUint32(r.sq.Head) != atomic.LoadUint32(r.sq.Tail)
}

// Close is used to close the ring.
func (r *Ring) Close() error {
	if err := r.closeSq(); err != nil {
		return err
	}
	if r.p.Flags&FeatSingleMmap == 0 {
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
	return int(atomic.LoadUint32(r.sq.Head) & atomic.LoadUint32(r.sq.Mask))
}

// SubmitTail returns the position of the tail of the submit queue. This method
// is safe for calling concurrently.
func (r *Ring) SubmitTail() int {
	return int(atomic.LoadUint32(r.sq.Tail) & atomic.LoadUint32(r.sq.Mask))
}

// CompleteHead returns the position of the head of the completion queue. This
// method is safe for calling concurrently.
func (r *Ring) CompleteHead() int {
	return int(atomic.LoadUint32(r.cq.Head) & atomic.LoadUint32(r.cq.Mask))
}

// CompleteTail returns the position of the tail of the submit queue. This method
// is safe for calling concurrently.
func (r *Ring) CompleteTail() int {
	return int(atomic.LoadUint32(r.cq.Tail) & atomic.LoadUint32(r.cq.Mask))
}

// SubmitEntry returns the next available SubmitEntry or nil if the ring is
// busy. The returned function should be called after SubmitEntry is ready to
// enter the ring.
func (r *Ring) SubmitEntry() (*SubmitEntry, func()) {
	// This function roughly follows this:
	// https://github.com/axboe/liburing/blob/master/src/queue.c#L258

getNext:
	tail := atomic.LoadUint32(r.sq.Tail)
	head := atomic.LoadUint32(r.sq.Head)
	mask := atomic.LoadUint32(r.sq.Mask)
	next := tail&mask + 1
	if next <= uint32(len(r.sq.Entries)) {
		// Make sure the ring is safe for updating by acquring the
		// update barrier.
		if !atomic.CompareAndSwapUint32(r.sq.Tail, tail, next) {
			runtime.Gosched()
			goto getNext
		}
		if atomic.LoadUint32(r.sq.entered) != 0 {
			runtime.Gosched()
			goto getNext
		}
		atomic.AddUint32(r.sq.writes, 1)

		// The callback that is returned is used to update the
		// state of the ring and decrement the active writes
		// counter.
		if r.debug {
			fmt.Printf("next: %d\nsq array:%+v\n", next, r.sq.Array[:5])
		}
		return &r.sq.Entries[tail&mask], func() {
			r.sq.completeWrite()
			r.sq.Array[next-1] = head & mask
		}
	}
	// When the ring wraps restart.
	atomic.CompareAndSwapUint32(r.sq.Tail, tail, 0)
	goto getNext
}

// ID returns an id for a SQEs, it is a monotonically increasing value (until
// uint64 wrapping).
func (r *Ring) ID() uint64 {
	return atomic.AddUint64(r.idx, 1)
}

// Fd returns the file descriptor of the ring.
func (r *Ring) Fd() int {
	return r.fd
}

// FileRegistry returns the FileRegistry for the Ring.
func (r *Ring) FileRegistry() FileRegistry {
	return r.fileReg
}

// FileReadWriter returns an io.ReadWriter from an os.File that uses the ring.
// Note that is is not valid to use other operations on the file (Seek/Close)
// in combination with the reader.
func (r *Ring) FileReadWriter(f *os.File) (ReadWriteSeekerCloser, error) {
	var offset int64
	if o, err := f.Seek(0, 0); err == nil {
		offset = int64(o)
	}
	rw := &ringFIO{
		r:       r,
		f:       f,
		fOffset: &offset,
	}
	if r.fileReg == nil {
		return rw, nil
	}
	return rw, r.fileReg.Register(int(f.Fd()))
}
