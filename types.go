// +build linux

package iouring

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
)

const (
	// CqSeenFlag is a nonstandard flag for handling concurrent readers
	// from the CompletionQueue.
	CqSeenFlag = 1
)

var (
	// ErrEntryNotFound is returned when a CQE is not found.
	ErrEntryNotFound = errors.New("Completion entry not found")

	errCQEMissing = errors.New("cqe missing")

	cqePool = sync.Pool{
		New: func() interface{} {
			return &CompletionEntry{}
		},
	}
)

type completionRequest struct {
	id    uint64
	res   int32
	flags uint32
	done  chan struct{}
}

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
	Flags    uint32
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
		if atomic.CompareAndSwapUint32(s.entered, 0, 1) {
			break
		}
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
	Flags    *uint32

	// Entries must never be resized, it is mmap'd.
	Entries []CompletionEntry
	ptr     uintptr
}

// Advance is used to advance the completion queue by a count.
func (c *CompletionQueue) Advance(count int) {
	atomic.AddUint32(c.Head, uint32(count))
}

// EntryBy (DEPRECATED) returns a CompletionEntry by comparing the user data,
// this should be called after the ring has been entered.
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
			seenIdx = i + 1
		}
		if cqe.UserData == userData {
			c.Entries[i].Flags |= CqSeenFlag
			if seenIdx == c.Size {
				seenIdx = 0
			}
			atomic.StoreUint32(c.Head, seenIdx)
			return &c.Entries[i], nil
		}
	}
	// Handle wrapping.
	seenIdx = uint32(0)
	seen = false
	seenEnd = false
	tail = atomic.LoadUint32(c.Tail)
	mask = atomic.LoadUint32(c.Mask)
	for i := uint32(0); i <= tail&mask; i++ {
		cqe := c.Entries[i]
		if cqe.Flags&CqSeenFlag == CqSeenFlag || cqe.IsZero() {
			seen = true
		} else if !seenEnd {
			seen = false
			seenEnd = true
		}
		if seen == true && !seenEnd {
			seenIdx = i + 1
		}
		if cqe.UserData == userData {
			c.Entries[i].Flags |= CqSeenFlag
			if seenIdx == c.Size {
				seenIdx = 0
			}
			atomic.StoreUint32(c.Head, seenIdx)
			return &c.Entries[i], nil
		}
	}

	return nil, ErrEntryNotFound
}
