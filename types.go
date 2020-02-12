package iouring

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

// SubmitQueue represents the submit queue ring buffer.
type SubmitQueue struct {
	Head        *uint
	Tail        *uint
	RingMask    *uint
	RingEntries *uint
	Flags       *uint
	Dropped     *uint

	Entries []SubmitEntry

	SqeHead *uint
	SqeTail *uint

	RingSize uint
	//ringPtr void
}

// CompletionEntry IO completion data structure (Completion Queue Entry).
type CompletionEntry struct {
	UserData uint64 /* sqe->data submission passed back */
	Res      int32  /* result code for this event */
	Flags    uint32
}

// CompletionQueue represents the completion queue ring buffer.
type CompletionQueue struct {
	Head        *uint
	Tail        *uint
	RingMask    *uint
	RingEntries *uint
	Overflow    *uint
	Entries     []CompletionEntry

	RingSize uint
	//ringPtr void
}
