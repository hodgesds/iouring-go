package iouring

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	// RingStateEmpty is when a ring is empty.
	RingStateEmpty uint32 = 1 << iota
	// RingStateUpdating is when a ring is preparing to be entered.
	RingStateUpdating
	// RingStateFilled is when a ring is filled and ready to be updated or
	// entered.
	RingStateFilled
	// RingStateWriting is when a ring is being written to.
	RingStateWriting
)

var (
	errEntryNotFound = errors.New("Completion entry not found")
)

// Params are used to configured a io uring.
type Params struct {
	SqEntries    uint32
	CqEntries    uint32
	Flags        uint32
	SqThreadCPU  uint32
	SqThreadIdle uint32
	Resv         [5]uint32
	SqOffset     SQRingOffset
	CqOffset     CQRingOffset
}

// CQRingOffset describes the various completion queue offsets.
type CQRingOffset struct {
	Head        uint32
	Tail        uint32
	RingMask    uint32
	RingEntries uint32
	Overflow    uint32
	Cqes        uint32
	Resv        [2]uint64
}

// SQRingOffset describes the various submit queue offets.
type SQRingOffset struct {
	Head        uint32
	Tail        uint32
	RingMask    uint32
	RingEntries uint32
	Flags       uint32
	Dropped     uint32
	Array       uint32
	Resv1       uint32
	Resv2       uint64
}

// SubmitEntry is an IO submission data structure (Submission Queue Entry).
type SubmitEntry struct {
	Opcode   Opcode /* type of operation for this sqe */
	Flags    uint8  /* IOSQE_ flags */
	Ioprio   uint16 /* ioprio for the request */
	Fd       int32  /* file descriptor to do IO on */
	Offset   uint64 /* offset into file */
	Addr     uint64 /* pointer to buffer or iovecs */
	Len      uint32 /* buffer size or number of iovecs */
	UFlags   int32
	UserData uint64
	Anon0    [24]byte /* extra padding */
}

// Reset is used to reset an SubmitEntry.
func (e *SubmitEntry) Reset() {
	e.Opcode = Nop
	e.Flags = 0
	e.Ioprio = 0
	e.Fd = -1
	e.Offset = 0
	e.Addr = 0
	e.Len = 0
	e.UFlags = 0
	e.UserData = 0
}

// SubmitQueue represents the submit queue ring buffer.
type SubmitQueue struct {
	Size    uint32
	Head    *uint32
	Tail    *uint32
	Mask    *uint32
	Flags   *uint32
	Dropped *uint32

	// Entries must never be resized, it is mmap'd.
	Entries []SubmitEntry
	// ptr is pointer to the start of the mmap.
	ptr uintptr

	// state is used for the state machine of the submit buffer for use as
	// a memory barrier.
	state *uint32
	// writes is used to keep track of the number of concurrent writers to
	// the ring.
	writes *uint32
}

// Reset is used to reset all entries.
func (s *SubmitQueue) Reset() {
	for _, entry := range s.Entries {
		entry.Reset()
	}
}

func (s *SubmitQueue) needWakeup() bool {
	return atomic.LoadUint32(s.Flags)&SqNeedWakeup != 0
}

func (s *SubmitQueue) startWrite() {
	atomic.AddUint32(s.writes, 1)
}

// completeWrite is used to signal that an entry in the map has been fully
// written.
func (s *SubmitQueue) completeWrite(idx uint32) {
	for {
		writes := atomic.LoadUint32(s.writes)
		if writes == 0 {
			panic("invalid number of sq write completions")
		}
		if atomic.CompareAndSwapUint32(s.writes, writes, writes-1) {
			if atomic.CompareAndSwapUint32(s.Tail, atomic.LoadUint32(s.Tail), idx) {
				return
			}
		}
		runtime.Gosched()
	}
}

// submitBarrier is used to prevent updating the submit queue while the queue
// is being entered.
func (s *SubmitQueue) submitBarrier() {
	for {
		switch state := atomic.LoadUint32(s.state); state {
		case RingStateWriting:
			// Concurrent writes are allowed.
			return
		case RingStateEmpty, RingStateFilled:
			if atomic.CompareAndSwapUint32(s.state, state, RingStateWriting) {
				return
			}
		case RingStateUpdating:
			runtime.Gosched()
		}
	}
}

// updateBarrier is used to wait for the ring to be in a state to be updated.
func (s *SubmitQueue) updateBarrier() {
	for {
		switch state := atomic.LoadUint32(s.state); state {
		case RingStateUpdating:
			return
		case RingStateEmpty, RingStateFilled:
			if atomic.CompareAndSwapUint32(s.state, state, RingStateUpdating) {
				return
			}
		case RingStateWriting:
		}
		runtime.Gosched()
	}
}

func (s *SubmitQueue) fill() {
	for {
		switch state := atomic.LoadUint32(s.state); state {
		case RingStateUpdating:
			// Wait for all writes to complete before filling.
			if writes := atomic.LoadUint32(s.writes); writes > 0 {
				runtime.Gosched()
				continue
			}
			if atomic.CompareAndSwapUint32(s.state, state, RingStateFilled) {
				atomic.AddUint32(s.Tail, 1)
				return
			}
		case RingStateWriting:
			if atomic.CompareAndSwapUint32(s.state, state, RingStateFilled) {
				return
			}
		}
		runtime.Gosched()
	}
}

// empty is used for transitioning the ring state after all submitted entries
// have been complete.
func (s *SubmitQueue) empty() {
	for {
		switch state := atomic.LoadUint32(s.state); state {
		case RingStateWriting:
			if atomic.CompareAndSwapUint32(s.state, state, RingStateEmpty) {
				return
			}
		default:
			panic(fmt.Sprintf(
				"can not transition to empty state from state %v",
				state,
			))
		}
	}
}

// CompletionEntry IO completion data structure (Completion Queue Entry).
type CompletionEntry struct {
	UserData uint64 /* sqe->data submission data passed back */
	Res      int32  /* result code for this event */
	Flags    uint32
}

// CompletionQueue represents the completion queue ring buffer.
type CompletionQueue struct {
	Size     uint32
	Head     *uint32
	Tail     *uint32
	Mask     *uint32
	Overflow *uint32

	// Entries must never be resized, it is mmap'd.
	Entries []CompletionEntry
	ptr     uintptr
}

// EntryBy returns a CompletionEntry by comparing the user data, this
// should be called after the ring has been entered.
func (c *CompletionQueue) EntryBy(userData uint64) (*CompletionEntry, error) {
	// TODO(hodges): This function is wrong but "should work", it
	// should follow this pattern:
	// To find the index of an event, the application must mask the current tail
	// index with the size mask of the ring.
	// ref: https://kernel.dk/io_uring.pdf

	head := atomic.LoadUint32(c.Head)
	tail := atomic.LoadUint32(c.Tail)
	mask := atomic.LoadUint32(c.Mask)

	// TODO: How should the head of the ring be updated with concurrent
	// callers?
	for i := head & mask; i < tail&mask; i++ {
		if c.Entries[i].UserData == userData {
			atomic.StoreUint32(c.Head, i+1)
			return &c.Entries[i], nil
		}
	}

	return nil, errEntryNotFound
}

// KernelTimespec is a kernel timespec.
type KernelTimespec struct {
	Sec  int64
	Nsec int64
}

// ReadWriteAtCloser supports reading, writing, and closing.
type ReadWriteAtCloser interface {
	io.WriterAt
	io.ReadWriteCloser
}

// ringFIO is used for handling file IO.
type ringFIO struct {
	r       *Ring
	f       *os.File
	fOffset *int64
}

// Read implements the io.Reader interface.
//go:nosplit
func (i *ringFIO) Read(b []byte) (int, error) {
	// TODO: Should nosplit be used in this case? The goal is to avoid
	// stack reallocations which could mess up some of the unsafe pointer
	// use.
	sqeID := i.r.Sqe() // index in the submit queue
	reqID := i.r.ID()

	// First the update barrier must be acquired.
	i.r.sq.updateBarrier()
	i.r.sq.startWrite()

	i.r.sq.Entries[sqeID].Reset()
	i.r.sq.Entries[sqeID].Opcode = ReadFixed
	i.r.sq.Entries[sqeID].Fd = int32(i.f.Fd())
	i.r.sq.Entries[sqeID].Len = uint32(len(b))
	i.r.sq.Entries[sqeID].Flags = uint8(SqeIoDrain)
	i.r.sq.Entries[sqeID].Offset = uint64(atomic.LoadInt64(i.fOffset))

	// This is probably a violation of the memory model, but in order for
	// reads to work we have to pass the address of the read buffer to the
	// SQE.
	i.r.sq.Entries[sqeID].Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	i.r.sq.Entries[sqeID].UserData = reqID

	i.r.sq.completeWrite(uint32(sqeID))
	// Once the updates are complete then transition to the filled state.
	// After the fill state transition then the ring is allowed to be
	// entered.
	i.r.sq.fill()

	// Submit the SQE by entering the ring.
	if i.r.debug {
		fmt.Printf("sq (pre) head: %v tail %v\nentries: %+v\n", *i.r.sq.Head, *i.r.sq.Tail, i.r.sq.Entries[:2])
		fmt.Printf("cq (pre) head: %v tail %v\nentries: %+v\n", *i.r.cq.Head, *i.r.cq.Tail, i.r.cq.Entries[:3])
	}
	if i.r.canEnter() {
		err := i.r.Enter(uint(1), uint(1), EnterGetEvents, nil)
		if err != nil {
			return 0, err
		}
	}

	// Use EntryBy to return the CQE by the "request" id in UserData.
	cqe, err := i.r.cq.EntryBy(reqID)
	if i.r.debug {
		fmt.Printf("sq head: %v tail %v\nentries: %+v\n", *i.r.sq.Head, *i.r.sq.Tail, i.r.sq.Entries[:2])
		fmt.Printf("cq  head: %v tail %v\nentries: %+v\n", *i.r.cq.Head, *i.r.cq.Tail, i.r.cq.Entries[:3])
	}
	if err != nil {
		return 0, err
	}
	if cqe.Res < 0 {
		return int(cqe.Res), fmt.Errorf("Error: %d", cqe.Res)
	}
	atomic.StoreInt64(i.fOffset, atomic.LoadInt64(i.fOffset)+int64(cqe.Res))

	return int(cqe.Res), nil
}

// Write implements the io.Writer interface.
//go:nosplit
func (i *ringFIO) Write(b []byte) (int, error) {
	sqeID := i.r.Sqe() // index in the submit queue
	reqID := i.r.ID()

	// First the update barrier must be acquired.
	i.r.sq.updateBarrier()
	i.r.sq.startWrite()

	i.r.sq.Entries[sqeID].Reset()
	i.r.sq.Entries[sqeID].Opcode = WriteFixed
	i.r.sq.Entries[sqeID].Fd = int32(i.f.Fd())
	i.r.sq.Entries[sqeID].Len = uint32(len(b))
	i.r.sq.Entries[sqeID].Flags = uint8(SqeIoDrain)
	i.r.sq.Entries[sqeID].Offset = uint64(atomic.LoadInt64(i.fOffset))

	// This is probably a violation of the memory model, but in order for
	// writes to work we have to pass the address of the write buffer to
	// the SQE.
	i.r.sq.Entries[sqeID].Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	i.r.sq.Entries[sqeID].UserData = reqID

	i.r.sq.completeWrite(uint32(sqeID))
	// Once the updates are complete then transition to the filled state.
	// After the fill state transition then the ring is allowed to be
	// entered.
	i.r.sq.fill()

	// Submit the SQE by entering the ring.
	if i.r.canEnter() {
		err := i.r.Enter(uint(1), uint(1), EnterGetEvents, nil)
		if err != nil {
			return 0, err
		}
	}

	// Use EntryBy to return the CQE by the "request" id in UserData.
	cqe, err := i.r.cq.EntryBy(reqID)
	if err != nil {
		return 0, err
	}
	if cqe.Res < 0 {
		return int(cqe.Res), fmt.Errorf("Error: %d", cqe.Res)
	}
	atomic.StoreInt64(i.fOffset, atomic.LoadInt64(i.fOffset)+int64(cqe.Res))

	return int(cqe.Res), nil
}

// WriteAt implements the io.WriterAt interface.
func (i *ringFIO) WriteAt(b []byte, o int64) (int, error) {
	return 0, errors.New("not implemented")
}

// Close implements the io.Closer interface.
func (i *ringFIO) Close() error {
	return i.f.Close()
}

// Seek implements the io.Seeker interface.
func (i *ringFIO) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		atomic.StoreInt64(i.fOffset, offset)
		return 0, nil
	case io.SeekCurrent:
		atomic.StoreInt64(i.fOffset, atomic.LoadInt64(i.fOffset)+offset)
		return 0, nil
	case io.SeekEnd:
		stat, err := i.f.Stat()
		if err != nil {
			return 0, err
		}
		atomic.StoreInt64(i.fOffset, stat.Size()-offset)
		return 0, nil
	default:
		return 0, errors.New("unknown whence")
	}
}
