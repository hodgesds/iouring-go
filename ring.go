// +build linux

package iouring

import (
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Ring contains an io_uring submit and completion ring.
type Ring struct {
	fd              int
	p               *Params
	cq              *CompletionQueue
	c               *completer
	cqMu            sync.RWMutex
	sq              *SubmitQueue
	sqMu            sync.RWMutex
	sqPool          sync.Pool
	idx             *uint64
	debug           bool
	fileReg         FileRegistry
	deadline        time.Duration
	enterErrHandler func(error)
	submitter       submitter

	stop           chan struct{}
	completions    chan *completionRequest
	eventFd        int
	completionPool sync.Pool
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
		p:           p,
		fd:          fd,
		cq:          &cq,
		sq:          &sq,
		idx:         &idx,
		fileReg:     nil,
		eventFd:     -1,
		stop:        make(chan struct{}, 32),
		completions: make(chan *completionRequest, len(cq.Entries)),
		c:           newCompleter(&cq, 512),
		completionPool: sync.Pool{
			New: func() interface{} {
				return &completionRequest{
					done: make(chan struct{}, 8),
				}
			},
		},
	}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	go r.run()
	go r.c.run()

	return r, nil
}

// CQ returns the CompletionQueue for the ring.
func (r *Ring) CQ() *CompletionQueue {
	return r.cq
}

// SQ returns the SubmitQueue for the ring.
func (r *Ring) SQ() *SubmitQueue {
	return r.sq
}

// EventFd returns the file descriptor of the eventfd if it is set, otherwise
// it returns the default value of -1.
func (r *Ring) EventFd() int {
	return r.eventFd
}

// Enter is used to enter the ring.
func (r *Ring) Enter(toSubmit uint, minComplete uint, flags uint, sigset *unix.Sigset_t) (int, error) {
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
	return completed, err
}

// run is used to run the ring and handle completions.
func (r *Ring) run() {
	inflight := map[uint64]*completionRequest{}
	retry := make(chan struct{}, 2)
	for {
		select {
		case <-r.stop:
			return
		case cr := <-r.completions:
			inflight[cr.id] = cr
			// TODO: Use the number completed for tracking
			count, err := r.Enter(uint(len(inflight)), 0, EnterGetEvents, nil)
			if err != nil {
				if r.enterErrHandler != nil {
					r.enterErrHandler(err)
				}
				// There still may be completed requests so continue on.
			}
			r.onEntry(inflight, count)
			if len(inflight) > 0 {
				retry <- struct{}{}
			}
		case <-retry:
			select {
			case cr := <-r.completions:
				inflight[cr.id] = cr
				_, err := r.Enter(uint(len(inflight)), 0, EnterGetEvents, nil)
				if err != nil {
					if r.enterErrHandler != nil {
						r.enterErrHandler(err)
					}
				}
			default:
			}
			r.onEntry(inflight, 0)
			if len(inflight) > 0 {
				// TODO: Use eventfd for polling instead.
				time.Sleep(200 * time.Nanosecond)
				retry <- struct{}{}
			}
		}
	}
}

func (r *Ring) complete(reqID uint64) (int32, uint32) {
	req := r.completionPool.Get().(*completionRequest)
	req.id = reqID
	req.res = 0
	req.flags = 0
	r.completions <- req
	<-req.done
	res := req.res
	flags := req.flags
	r.completionPool.Put(req)
	return res, flags
}

func (r *Ring) onEntry(inflight map[uint64]*completionRequest, count int) {
	mask := atomic.LoadUint32(r.cq.Mask)
	head := atomic.LoadUint32(r.cq.Head)
	tail := atomic.LoadUint32(r.cq.Tail)
	nEntries := uint32(len(r.cq.Entries))
	seenIdx := uint32(0)
	seen := true
	for i := head & mask; i < nEntries; i++ {
		cqe := r.cq.Entries[i]
		if cr, ok := inflight[cqe.UserData]; ok {
			if seen {
				seenIdx++
			}
			cr.res = cqe.Res
			cr.flags = cqe.Flags
			cr.done <- struct{}{}
			delete(inflight, cr.id)
		} else {
			seen = false
		}
		if i == tail&mask {
			atomic.StoreUint32(r.cq.Head, head+seenIdx)
			return
		}
	}
	seen = true
	for i := uint32(0); i < tail&mask; i++ {
		cqe := r.cq.Entries[i]
		if cr, ok := inflight[cqe.UserData]; ok {
			if seen {
				seenIdx++
			}
			cr.res = cqe.Res
			cr.flags = cqe.Flags
			cr.done <- struct{}{}
			delete(inflight, cr.id)
		} else {
			seen = false
		}
	}
	atomic.StoreUint32(r.cq.Head, head+seenIdx)
}

// getCqe is used for getting a CQE result.
func (r *Ring) getCqe(reqID uint64) (int32, uint32, error) {
	cq := r.cq
findCqe:

	head := atomic.LoadUint32(cq.Head)
	tail := atomic.LoadUint32(cq.Tail)
	mask := atomic.LoadUint32(cq.Mask)
	end := int(tail & mask)

	for x := int(head & mask); x < len(cq.Entries); x++ {
		cqe := cq.Entries[x]
		if cqe.UserData == reqID {
			if cqe.Res < 0 {
				return 0, 0, syscall.Errno(-cqe.Res)
			}
			return cqe.Res, cqe.Flags, nil
		}
		if x == end {
			goto findCqe
			return 0, 0, errCQEMissing
		}
	}
	tail = atomic.LoadUint32(cq.Tail)
	mask = atomic.LoadUint32(cq.Mask)
	end = int(tail & mask)
	for x := 0; x < end; x++ {
		cqe := cq.Entries[x]
		if cqe.UserData == reqID {
			if cqe.Res < 0 {
				return 0, 0, syscall.Errno(-cqe.Res)
			}
			return cqe.Res, cqe.Flags, nil
		}
		if x == end {
			goto findCqe
			return 0, 0, errCQEMissing
		}
	}
	return 0, 0, errCQEMissing
}

// CanEnter returns whether or not the ring can be entered.
func (r *Ring) CanEnter() bool {
	// TODO: figure out this
	return true
}

// ShouldFlush returns if the ring should flush due to cq being overflown.
func (r *Ring) ShouldFlush() bool {
	return atomic.LoadUint32(r.sq.Flags)&SqCqOverflow != 0
}

// NeedsEnter returns if the ring needs to be entered.
func (r *Ring) NeedsEnter() bool {
	return atomic.LoadUint32(r.sq.Flags)&SqNeedWakeup != 0
}

// Stop is used to stop the ring.
func (r *Ring) Stop() error {
	if err := r.closeSq(); err != nil {
		return err
	}
	if r.p.Flags&FeatSingleMmap == 0 {
		if err := r.closeCq(); err != nil {
			return err
		}
	}
	if r.submitter != nil {
		r.submitter.stop()
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

		r.sq.Entries[tail&mask].Reset()
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
	return r.fileReadWriter(f)
}

func (r *Ring) fileReadWriter(f *os.File) (*ringFIO, error) {
	var offset int64
	o, err := f.Seek(0, 0)
	if err != nil {
		return nil, err
	}
	offset = int64(o)
	rw := &ringFIO{
		r:       r,
		f:       f,
		fd:      int32(f.Fd()),
		fOffset: &offset,
		c:       r.c,
	}
	if r.fileReg == nil {
		return rw, nil
	}
	return rw, r.fileReg.Register(int(f.Fd()))
}
