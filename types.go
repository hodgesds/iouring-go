package iouring

import (
	"io"
	"os"
	"sync"
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
	Off      uint64 /* offset into file */
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
	e.Off = 0
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
}

// CompletionEntry IO completion data structure (Completion Queue Entry).
type CompletionEntry struct {
	UserData uint64 /* sqe->data submission passed back */
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
	sqe := i.r.Sqe()
	i.r.sq.Entries[sqe].Reset()
	i.r.sq.Entries[sqe].Opcode = ReadFixed
	i.r.sq.Entries[sqe].Fd = int32(i.f.Fd())
	i.r.sq.Entries[sqe].UserData = i.r.Idx()
	i.r.sq.Entries[sqe].Len = uint32(len(b))

	return 0, nil
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
