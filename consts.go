// +build linux

package iouring

import "io"

// See uapi/linux/io_uring.h

// Opcode is an opcode for the ring.
type Opcode uint8

const (
	// SetupSyscall defines the syscall number for io_uring_setup.
	SetupSyscall = 425
	// EnterSyscall defines the syscall number for io_uring_enter.
	EnterSyscall = 426
	// RegisterSyscall defines the syscall number for io_uring_register.
	RegisterSyscall = 427
)

const (

	// FeatSingleMmap is used to configure a single mmap'd ring.
	FeatSingleMmap = (1 << 0)
	// FeatNoDrop is used to ensure that no CQEs are dropped.
	FeatNoDrop         = (1 << 1)
	FeatSubmitStable   = (1 << 2)
	FeatRwCurPos       = (1 << 3)
	FeatCurPersonality = (1 << 4)
)

const (
	/*
	 * sqe->flags
	 */
	SqeFixedFileBit = iota
	SqeIoDrainBit
	SqeIoLinkBit
	SqeIoHardlinkBit
	SqeAsyncBit
	SqeBufferSelectBit

	// SqeFixedFile use fixed fileset
	SqeFixedFile uint8 = (1 << SqeFixedFileBit)
	// SqeIoDrain issue after inflight IO
	SqeIoDrain uint8 = (1 << SqeIoDrainBit)
	// SqeIoLink is used to link multiple SQEs.
	SqeIoLink uint8 = (1 << SqeIoLinkBit)
	// SqeIoHardlink is a hard link to multiple SQEs
	SqeIoHardlink uint8 = (1 << SqeIoHardlinkBit)
	// SqeAsync is use to specify async io.
	SqeAsync uint8 = (1 << SqeAsyncBit)
	// SqeBufferSelect is used to specify buffer select.
	SqeBufferSelect uint8 = (1 << SqeBufferSelectBit)

	/*
	 * io_uring_setup() flags
	 */

	// SetupIOPoll io_context is polled
	SetupIOPoll uint32 = (1 << 0)
	// SetupSQPoll SQ poll thread
	SetupSQPoll uint32 = (1 << 1)
	// SetupSQAFF sq_thread_cpu is valid
	SetupSQAFF uint32 = (1 << 2)
	// SetupCqSize app defines CQ size
	SetupCqSize uint32 = (1 << 3)
	// SetupClamp clamp SQ/CQ ring sizes
	SetupClamp uint32 = (1 << 4)
	// SetupAttachWq  attach to existing wq
	SetupAttachWq uint32 = (1 << 5)
)

const (
	Nop Opcode = iota
	Readv
	Writev
	Fsync
	ReadFixed
	WriteFixed
	PollAdd
	PollRemove
	SyncFileRange
	SendMsg
	RecvMsg
	Timeout
	TimeoutRemove
	Accept
	AsyncCancel
	LinkTimeout
	Connect
	Fallocate
	OpenAt
	Close
	FilesUpdate
	Statx
	Read
	Write
	Fadvise
	Madvise
	Send
	Recv
	Openat2
	EpollCtl
	Splice
	ProvideBuffers
	RemoveBuffers
	OpSupported = (1 << 0)
)
const (
	/*
	 * sqe->fsync_flags
	 */

	// FsyncDatasync ...
	FsyncDatasync uint = (1 << 0)

	/*
	 * Magic offsets for the application to mmap the data it needs
	 */

	// SqRingOffset is the offset of the submission queue.
	SqRingOffset uint64 = 0
	// CqRingOffset is the offset of the completion queue.
	CqRingOffset uint64 = 0x8000000
	// SqeRingOffset is the offset of the submission queue entries.
	SqeRingOffset uint64 = 0x10000000

	/*
	 * sq_ring->flags
	 */

	// SqNeedWakeup needs io_uring_enter wakeup
	SqNeedWakeup uint32 = (1 << 0)
	SqCqOverflow uint32 = (1 << 1)

	/*
	 * io_uring_enter(2) flags
	 */

	// EnterGetEvents ...
	EnterGetEvents uint = (1 << 0)
	// EnterSqWakeup ...
	EnterSqWakeup uint = (1 << 1)

	/*
	 * io_uring_register(2) opcodes and arguments
	 */

	RegRegisterBuffers       = 0
	RegUnregisterBuffers     = 1
	RegRegisterFiles         = 2
	RegUnregisterFiles       = 3
	RegRegisterEventFd       = 4
	RegUnregisterEventfd     = 5
	RegRegisterFilesUpdate   = 6
	RegRegisterEventFdAsync  = 7
	RegRegisterProbe         = 8
	RegRegisterPersonality   = 9
	RegUnregisterPersonality = 10
)

// ReadWriteSeekerCloser is a ReadWriteCloser and ReadWriteSeeker.
type ReadWriteSeekerCloser interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
	ReadAt([]byte, int64) (int, error)
	WriteAt([]byte, int64) (int, error)
}
