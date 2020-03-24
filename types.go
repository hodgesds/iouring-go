package iouring

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
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
	UFlags   uint32 /* union of various flags */
	UserData uint64 /* data to be passed back at completion time */
	BufIndex uint16 /* index into fixed buffers, if used */
}

// Reset is used to reset an SubmitEntry.
func (e *SubmitEntry) Reset() {
	e.Flags = 0
	e.Ioprio = 0
	e.Fd = -1
	e.Offset = 0
	e.Addr = 0
	e.Len = 0
	e.UFlags = 0
	e.UserData = 0
	e.BufIndex = 0
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
	Entries   []SubmitEntry
	entriesMu sync.RWMutex
	// ptr is pointer to the start of the mmap.
	ptr uintptr

	state *uint32
}

func (s *SubmitQueue) needWakeup() bool {
	return atomic.LoadUint32(s.Flags)&SqNeedWakeup != 0
}

// submitBarrier is used to prevent updating the submit queue while the queue
// is being entered.
func (s *SubmitQueue) submitBarrier() {
	for {
		switch state := atomic.LoadUint32(s.state); state {
		case RingStateWriting:
			// Should two submits to the ring be allowed?
			return
		case RingStateEmpty, RingStateFilled:
			if atomic.CompareAndSwapUint32(s.state, state, RingStateWriting) {
				return
			}
		case RingStateUpdating:
		}
		runtime.Gosched()
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
		case RingStateUpdating, RingStateWriting:
			if atomic.CompareAndSwapUint32(s.state, state, RingStateFilled) {
				return
			}
		default:
			runtime.Gosched()
			continue
		}
	}
}

func (s *SubmitQueue) empty() {
	for {
		state := atomic.LoadUint32(s.state)
		if state != RingStateWriting {
			runtime.Gosched()
			continue
		}
		if atomic.CompareAndSwapUint32(s.state, state, RingStateEmpty) {
			return
		}
	}
}

// CompletionEntry IO completion data structure (Completion Queue Entry).
type CompletionEntry struct {
	UserData uint64 /* sqe->data submission passed back, as a pointer... */
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
	Entries   []CompletionEntry
	entriesMu sync.RWMutex
	// ptr is pointer to the start of the mmap.
	ptr uintptr
}

// EntryBy returns.
func (c *CompletionQueue) EntryBy(idx uint64) (CompletionEntry, error) {
	// TODO(hodges): This function is wrong but "should work", it
	// should follow this pattern:
	// To find the index of an event, the application must mask the current tail
	// index with the size mask of the ring.
	// ref: https://kernel.dk/io_uring.pdf

	for _, entry := range c.Entries {
		if entry.UserData == idx {
			return entry, nil
		}
		//if entry.UserData != 0 {
		//	fmt.Printf("%+v\n", entry)
		//}
	}
	return CompletionEntry{}, errEntryNotFound
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
	fOffset *uint64
}

// Read implements the io.Reader interface.
func (i *ringFIO) Read(b []byte) (int, error) {
	sqeIdx := i.r.Sqe()
	reqIdx := i.r.Idx()

	// First the update barrier must be acquired.
	i.r.sq.updateBarrier()

	i.r.sq.Entries[sqeIdx].Reset()
	i.r.sq.Entries[sqeIdx].Opcode = Read
	i.r.sq.Entries[sqeIdx].Fd = int32(i.f.Fd())
	i.r.sq.Entries[sqeIdx].UserData = (uint64)(uintptr(unsafe.Pointer(&reqIdx)))
	i.r.sq.Entries[sqeIdx].Len = uint32(len(b))
	//i.r.sq.Entries[sqeIdx].Flags = uint8(SqeIoDrain)
	i.r.sq.Entries[sqeIdx].Offset = atomic.LoadUint64(i.fOffset)
	// This is probably a violation of the memory model.
	i.r.sq.Entries[sqeIdx].Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	fmt.Printf("%+v\n", i.r.sq.Entries[sqeIdx])

	// Once the updates are complete then transition to the filled state.
	// After the fill state transition then the ring is allowed to be
	// entered.
	i.r.sq.fill()

	// Submit the SQE by entering the ring.
	err := i.r.Enter(uint(1), uint(1), EnterGetEvents, nil)
	if err != nil {
		return 0, err
	}

	reqIdxPtr := (uint64)(uintptr(unsafe.Pointer(&reqIdx)))
	cqe, err := i.r.cq.EntryBy(reqIdxPtr)
	// Handle these errors better.
	if err != nil {
		return 0, nil //err
	}
	if cqe.Res < 0 {
		return int(cqe.Res), fmt.Errorf("Error: %d", cqe.Res)
	}

	return int(cqe.Res), nil
}

// Write implements the io.Writer interface.
func (i *ringFIO) Write(b []byte) (int, error) {
	return 0, nil
}

// WriteAt implements the io.WriterAt interface.
func (i *ringFIO) WriteAt(b []byte, o int64) (int, error) {
	return 0, nil
}

// Close implements the io.Closer interface.
func (i *ringFIO) Close() error {
	return i.f.Close()
}
