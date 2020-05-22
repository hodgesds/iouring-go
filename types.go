// +build linux

package iouring

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// RingState is the state of the ring.
type RingState uint32

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

	// CqSeenFlag is a nonstandard flag for handling concurrent readers
	// from the CompletionQueue.
	CqSeenFlag = 1
)

// String implements the stringer method.
func (s RingState) String() string {
	switch uint32(s) {
	case RingStateEmpty:
		return "empty"
	case RingStateUpdating:
		return "updating"
	case RingStateFilled:
		return "filled"
	case RingStateWriting:
		return "writing"
	default:
		return "invalid"
	}
}

var (
	// ErrEntryNotFound is returned when a CQE is not found.
	ErrEntryNotFound = errors.New("Completion entry not found")
)

// Params are used to configured a io uring.
type Params struct {
	SqEntries    uint32
	CqEntries    uint32
	Flags        uint32
	SqThreadCPU  uint32
	SqThreadIdle uint32
	Features     uint32
	WqFD         uint32
	Resv         [3]uint32
	SqOffset     SQRingOffset
	CqOffset     CQRingOffset
}

// SQRingOffset describes the various submit queue offsets.
type SQRingOffset struct {
	Head     uint32
	Tail     uint32
	RingMask uint32
	Entries  uint32
	Flags    uint32
	Dropped  uint32
	Array    uint32
	Resv1    uint32
	Resv2    uint64
}

// CQRingOffset describes the various completion queue offsets.
type CQRingOffset struct {
	Head     uint32
	Tail     uint32
	RingMask uint32
	Entries  uint32
	Overflow uint32
	Cqes     uint32
	Resv     [2]uint64
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

	// Array holds entries to be submitted; it must never be resized it is mmap'd.
	Array []uint32
	// Entries must never be resized, it is mmap'd.
	Entries []SubmitEntry
	// ptr is pointer to the start of the mmap.
	ptr uintptr

	// entered is when the ring is being entered.
	entered *uint32
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

// NeedWakeup is used to determine whether the submit queue needs awoken.
func (s *SubmitQueue) NeedWakeup() bool {
	return atomic.LoadUint32(s.Flags)&SqNeedWakeup != 0
}

func (s *SubmitQueue) enterLock() {
	for {
		if atomic.LoadUint32(s.writes) != 0 && atomic.LoadUint32(s.entered) == 1 {
			runtime.Gosched()
			continue
		}
		atomic.StoreUint32(s.entered, 1)
		break
	}
}

func (s *SubmitQueue) enterUnlock() {
	atomic.StoreUint32(s.entered, 0)
}

// completeWrite is used to signal that an entry in the map has been fully
// written.
func (s *SubmitQueue) completeWrite() {
	for {
		writes := atomic.LoadUint32(s.writes)
		if writes == 0 {
			panic("invalid number of sq write completions")
		}
		if atomic.CompareAndSwapUint32(s.writes, writes, writes-1) {
			return
		}
		runtime.Gosched()
	}
}

// CompletionEntry IO completion data structure (Completion Queue Entry).
type CompletionEntry struct {
	UserData uint64 /* sqe->data submission data passed back */
	Res      int32  /* result code for this event */
	Flags    uint32
}

// IsZero returns if the CQE is zero valued.
func (c *CompletionEntry) IsZero() bool {
	return c.UserData == 0 && c.Res == 0 && c.Flags == 0
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
	head := atomic.LoadUint32(c.Head)
	tail := atomic.LoadUint32(c.Tail)
	mask := atomic.LoadUint32(c.Mask)
	if head&mask == tail&mask {
		return nil, ErrEntryNotFound
	}

	// seenIdx is used for indicating the largest consecutive seen CQEs,
	// which is then used for setting the new head position. This is done
	// by setting the CqSeenFlag bit on a CQE UserData once a CQE has been
	// read. The head is then set to the largest consecutive seen index.
	seenIdx := head & mask
	seen := false
	seenEnd := false
	for i := seenIdx; i <= uint32(len(c.Entries)-1); i++ {
		cqe := c.Entries[i]
		if cqe.Flags&CqSeenFlag == CqSeenFlag || cqe.IsZero() {
			seen = true
		} else if !seenEnd {
			seen = false
			seenEnd = true
		}
		if seen == true && !seenEnd {
			seenIdx = i
		}
		if c.Entries[i].UserData == userData {
			c.Entries[i].Flags |= CqSeenFlag
			atomic.StoreUint32(c.Head, seenIdx)
			return &c.Entries[i], nil
		}
	}
	// Handle wrapping.
	seenIdx = uint32(0)
	seen = false
	seenEnd = false
	for i := uint32(0); i <= tail&mask; i++ {
		cqe := c.Entries[i]
		if cqe.Flags&CqSeenFlag == CqSeenFlag || cqe.IsZero() {
			seen = true
		} else if !seenEnd {
			seen = false
			seenEnd = true
		}
		if seen == true && !seenEnd {
			seenIdx = i
		}
		if c.Entries[i].UserData == userData {
			c.Entries[i].Flags |= CqSeenFlag
			atomic.StoreUint32(c.Head, seenIdx)
			return &c.Entries[i], nil
		}
	}

	return nil, ErrEntryNotFound
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

// getCqe is used for getting a CQE result and will retry up to one time.
func (i *ringFIO) getCqe(reqID uint64, retryNotFound bool) (int, error) {
	if i.r.CanEnter() {
		_, err := i.r.Enter(uint(1), uint(1), EnterGetEvents, nil)
		if err != nil {
			return 0, err
		}
		if i.r.debug {
			fmt.Printf("enter complete\n")
		}
	}
	if i.r.debug {
		sqHead := *i.r.sq.Head
		sqTail := *i.r.sq.Tail
		sqMask := *i.r.sq.Mask
		cqHead := *i.r.cq.Head
		cqTail := *i.r.cq.Tail
		cqMask := *i.r.cq.Mask
		fmt.Printf("sq head: %v tail: %v\nsq entries: %+v\n", sqHead&sqMask, sqTail&sqMask, i.r.sq.Entries)
		fmt.Printf("sq array: %+v\n", i.r.sq.Array)
		fmt.Printf("cq head: %v tail: %v\ncq entries: %+v\n", cqHead&cqMask, cqTail&cqMask, i.r.cq.Entries)
	}

	// Use EntryBy to return the CQE by the "request" id in UserData.
	cqe, err := i.r.cq.EntryBy(reqID)
	if err != nil {
		if err == ErrEntryNotFound && retryNotFound {
			return i.getCqe(reqID, false)
		}
		return 0, err
	}
	if cqe.Res < 0 {
		return int(cqe.Res), syscall.Errno(cqe.Res)
	}
	atomic.StoreInt64(i.fOffset, atomic.LoadInt64(i.fOffset)+int64(cqe.Res))

	return int(cqe.Res), nil
}

// Write implements the io.Writer interface.
//go:nosplit
func (i *ringFIO) Write(b []byte) (int, error) {
	sqe, ready := i.r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = WriteFixed
	sqe.Fd = int32(i.f.Fd())
	sqe.Len = uint32(len(b))
	sqe.Flags = 0
	sqe.Offset = uint64(atomic.LoadInt64(i.fOffset))

	// This is probably a violation of the memory model, but in order for
	// reads to work we have to pass the address of the read buffer to the
	// SQE.
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	reqID := i.r.ID()
	sqe.UserData = reqID

	// Call the callback to signal we are ready to enter the ring.
	ready()

	return i.getCqe(reqID, true)
}

// Read implements the io.Reader interface.
//go:nosplit
func (i *ringFIO) Read(b []byte) (int, error) {
	sqe, ready := i.r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = ReadFixed
	sqe.Fd = int32(i.f.Fd())
	sqe.Len = uint32(len(b))
	sqe.Flags = 0
	sqe.Offset = uint64(atomic.LoadInt64(i.fOffset))

	// This is probably a violation of the memory model, but in order for
	// reads to work we have to pass the address of the read buffer to the
	// SQE.
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	reqID := i.r.ID()
	sqe.UserData = reqID

	// Call the callback to signal we are ready to enter the ring.
	ready()

	if i.r.debug {
		sqHead := *i.r.sq.Head
		sqTail := *i.r.sq.Tail
		sqMask := *i.r.sq.Mask
		cqHead := *i.r.cq.Head
		cqTail := *i.r.cq.Tail
		cqMask := *i.r.cq.Mask
		fmt.Printf("pre enter\n")
		fmt.Printf("sq head: %v tail: %v\nsq entries: %+v\n", sqHead&sqMask, sqTail&sqMask, i.r.sq.Entries)
		fmt.Printf("cq head: %v tail: %v\ncq entries: %+v\n", cqHead&cqMask, cqTail&cqMask, i.r.cq.Entries)
	}

	return i.getCqe(reqID, true)
}

// WriteAt implements the io.WriterAt interface.
func (i *ringFIO) WriteAt(b []byte, o int64) (int, error) {
	sqe, ready := i.r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = WriteFixed
	sqe.Fd = int32(i.f.Fd())
	sqe.Len = uint32(len(b))
	sqe.Flags = 0
	sqe.Offset = uint64(o)

	// This is probably a violation of the memory model, but in order for
	// reads to work we have to pass the address of the read buffer to the
	// SQE.
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	reqID := i.r.ID()
	sqe.UserData = reqID

	// Call the callback to signal we are ready to enter the ring.
	ready()

	return i.getCqe(reqID, true)
}

// ReadAt implements the io.ReaderAt interface.
func (i *ringFIO) ReadAt(b []byte, o int64) (int, error) {
	sqe, ready := i.r.SubmitEntry()
	if sqe == nil {
		return 0, errors.New("ring unavailable")
	}

	sqe.Opcode = ReadFixed
	sqe.Fd = int32(i.f.Fd())
	sqe.Len = uint32(len(b))
	sqe.Flags = 0
	sqe.Offset = uint64(o)

	// This is probably a violation of the memory model, but in order for
	// reads to work we have to pass the address of the read buffer to the
	// SQE.
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	// Use reqId as user data so we can return the request from the
	// completion queue.
	reqID := i.r.ID()
	sqe.UserData = reqID

	// Call the callback to signal we are ready to enter the ring.
	ready()

	return i.getCqe(reqID, true)
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
