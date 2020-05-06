package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

const (
	// SetupSyscall defines the syscall number for io_uring_setup.
	SetupSyscall = 425
	// EnterSyscall defines the syscall number for io_uring_enter.
	EnterSyscall = 426
	// RegisterSyscall defines the syscall number for io_uring_register.
	RegisterSyscall = 427

	// SqRingOffset is the offset of the submission queue.
	SqRingOffset uint64 = 0
	// CqRingOffset is the offset of the completion queue.
	CqRingOffset uint64 = 0x8000000
	// SqeRingOffset is the offset of the submission queue entries.
	SqeRingOffset uint64 = 0x10000000

	// EnterGetEvents if the bit is set in flags, then the system call will
	// attempt to wait for min_complete event completions before
	// returning.
	EnterGetEvents uint = (1 << 0)

	// Opcodes
	Read uint8 = 22
)

var (
	uint32Size = unsafe.Sizeof(uint32(0))
	cqeSize    = unsafe.Sizeof(Cqe{})
	sqeSize    = unsafe.Sizeof(Sqe{})
)

type Sqe struct {
	Opcode    uint8
	Flags     uint8
	Ioprio    uint16
	Fd        int32
	Off       uint64
	Addr      uint64
	Len       uint32
	Rw_flags  int32
	User_data uint64
	Anon0     [24]byte
}

type Cqe struct {
	Data  uint64
	Res   int32
	Flags uint32
}

type SqOffset struct {
	Head    uint32
	Tail    uint32
	Mask    uint32
	Entries uint32
	Flags   uint32
	Dropped uint32
	Array   uint32
	Resv1   uint32
	Resv2   uint64
}

type CqOffset struct {
	Head     uint32
	Tail     uint32
	Mask     uint32
	Entries  uint32
	Overflow uint32
	Cqes     uint32
	Resv     [2]uint64
}

type Params struct {
	Sq_entries     uint32
	Cq_entries     uint32
	Flags          uint32
	Sq_thread_cpu  uint32
	Sq_thread_idle uint32
	Features       uint32
	Wq_fd          uint32
	Resv           [3]uint32
	Sq_off         SqOffset
	Cq_off         CqOffset
}

type SubmitQueue struct {
	Size    uint32
	Head    *uint32
	Tail    *uint32
	Mask    *uint32
	Flags   *uint32
	Dropped *uint32

	// Entries must never be resized, it is mmap'd.
	Entries []Sqe
	Array   []uint32
}

type CompletionQueue struct {
	Size     uint32
	Head     *uint32
	Tail     *uint32
	Mask     *uint32
	Overflow *uint32

	// Entries must never be resized, it is mmap'd.
	Entries []Cqe
}

// Setup is used to setup a io_uring using the io_uring_setup syscall.
func Setup(entries uint, params *Params) (int, error) {
	fd, _, errno := syscall.Syscall(
		SetupSyscall,
		uintptr(entries),
		uintptr(unsafe.Pointer(params)),
		uintptr(0),
	)
	// errno is a special type in Go, see:
	// https://golang.org/pkg/syscall/#Errno
	if errno != 0 {
		err := errno
		return 0, err
	}
	return int(fd), nil
}

// MmapRing is used to configure the submit and completion queues, it should
// only be called after the Setup function has completed successfully.
// See:
// https://github.com/axboe/liburing/blob/master/src/setup.c#L22
func MmapRing(fd int, p *Params, sq *SubmitQueue, cq *CompletionQueue) error {
	var (
		cqPtr uintptr
		sqPtr uintptr
		errno syscall.Errno
		err   error
	)
	sq.Size = uint32(uint(p.Sq_off.Array) + (uint(p.Sq_entries) * uint(uint32Size)))

	sqPtr, _, errno = syscall.Syscall6(
		syscall.SYS_MMAP,
		uintptr(0),
		uintptr(sq.Size),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(fd),
		uintptr(SqRingOffset),
	)
	if errno != 0 {
		err = errno
		return err
	}

	// Conversion of a uintptr back to Pointer is not valid in general,
	// except for:
	// 3) Conversion of a Pointer to a uintptr and back, with arithmetic.

	// go vet doesn't like these casts so it probably violates the memory
	// model.
	sq.Head = (*uint32)(unsafe.Pointer(sqPtr + uintptr(p.Sq_off.Head)))
	sq.Tail = (*uint32)(unsafe.Pointer(sqPtr + uintptr(p.Sq_off.Tail)))
	sq.Mask = (*uint32)(unsafe.Pointer(sqPtr + uintptr(p.Sq_off.Mask)))
	sq.Flags = (*uint32)(unsafe.Pointer(sqPtr + uintptr(p.Sq_off.Flags)))
	sq.Dropped = (*uint32)(unsafe.Pointer(sqPtr + uintptr(p.Sq_off.Dropped)))

	// Map the sqe ring.
	sqePtr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		uintptr(0),
		uintptr(uint(p.Sq_entries)*uint(sqeSize)),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(fd),
		uintptr(SqeRingOffset),
	)
	if errno != 0 {
		err = errno
		return err
	}

	// Making mmap'd slices requires doing an unsafe pointer cast.
	sq.Entries = *(*[]Sqe)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(sqePtr),
		Len:  int(p.Sq_entries),
		Cap:  int(p.Sq_entries),
	}))
	sq.Array = *(*[]uint32)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(sqPtr + uintptr(p.Sq_off.Array))),
		Len:  int(p.Sq_entries),
		Cap:  int(p.Sq_entries),
	}))

	// Map the completion queue.
	cq.Size = uint32(uint(p.Cq_off.Cqes) + (uint(p.Cq_entries) * uint(cqeSize)))
	cqPtr, _, errno = syscall.Syscall6(
		syscall.SYS_MMAP,
		uintptr(0),
		uintptr(cq.Size),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(fd),
		uintptr(CqRingOffset),
	)
	if errno != 0 {
		err = errno
		return err
	}

	cq.Head = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.Cq_off.Head))))
	cq.Tail = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.Cq_off.Tail))))
	cq.Mask = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.Cq_off.Mask))))
	cq.Overflow = (*uint32)(unsafe.Pointer(uintptr(uint(cqPtr) + uint(p.Cq_off.Overflow))))

	cq.Entries = *(*[]Cqe)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(uint(cqPtr) + uint(p.Cq_off.Cqes)),
		Len:  int(p.Cq_entries),
		Cap:  int(p.Cq_entries),
	}))

	return nil
}

// Enter is used to submit to the queue.
func Enter(fd int, toSubmit uint, minComplete uint, flags uint /* sigset *unix.Sigset_t*/) (int, error) {
	res, _, errno := syscall.Syscall6(
		EnterSyscall,
		uintptr(fd),
		uintptr(toSubmit),
		uintptr(minComplete),
		uintptr(flags),
		/*uintptr(unsafe.Pointer(sigset)),*/
		uintptr(0),
		uintptr(0),
	)
	if errno != 0 {
		var err error
		err = errno
		return 0, err
	}

	return int(res), nil
}

func main() {
	// First create a tempfile for writing some test data.
	tmpFile, err := ioutil.TempFile("", "example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Write to the file without using the ring.
	content := []byte("testing  1,2,3")
	_, err = tmpFile.Write(content)
	if err != nil {
		log.Fatal(err)
	}

	_, err = tmpFile.Seek(0, 0)
	if err != nil {
		log.Fatal(err)
	}

	p := &Params{}
	ringFd, err := Setup(8, p)
	if err != nil {
		log.Fatal(err)
	}
	var (
		cq CompletionQueue
		sq SubmitQueue
	)
	if err := MmapRing(ringFd, p, &sq, &cq); err != nil {
		log.Fatal(err)
	}

	offset := uint64(0)
	exContent := []byte{}

	// Do two reads to read the content from the tempfile.
	for i := 0; i < 2; i++ {
		readBuff := make([]byte, len(content)/2)
		sqTail := *sq.Tail
		sqIdx := sqTail & *sq.Mask

		// Prepare the Sqe
		sq.Entries[sqIdx].Opcode = Read
		sq.Entries[sqIdx].Fd = int32(tmpFile.Fd())
		sq.Entries[sqIdx].Off = offset
		sq.Entries[sqIdx].Len = uint32(len(readBuff))
		sq.Entries[sqIdx].User_data = uint64(i + 1)

		// This is probably a violation of the memory model, but in
		// order for reads to work we have to pass the address of the
		// read buffer to the SQE. If the readBuffer is heap allocated
		// then it is less of a problem.
		sq.Entries[sqIdx].Addr = (uint64)(uintptr(unsafe.Pointer(&readBuff[0])))

		sq.Array[sqIdx] = *sq.Head & *sq.Mask
		*sq.Tail += 1
		fmt.Printf("sq head:%v tail: %v\nsq entries: %+v\n", *sq.Head, *sq.Tail, sq.Entries[:2])

		fmt.Println("entering the ring")
		n, err := Enter(ringFd, uint(1), uint(1), EnterGetEvents)
		if err != nil {
			log.Fatal(err)
		}
		if n != 1 {
			log.Fatalf("expected 1 completed entry, got: %v", n)
		}

		cqTail := *cq.Tail
		cqHead := *cq.Head
		if cqHead == cqTail {
			log.Fatal("No entries\n")
		}
		fmt.Printf("cq head:%v tail: %v\ncq entries: %+v\n", *cq.Head, *cq.Tail, cq.Entries[:2])

		// Search for the cqe in a suboptimal manner
		for cqIdx := cqHead & *cq.Mask; cqIdx < cqTail; cqIdx++ {
			// The Cqe data should match our loop index (i)+1
			if cq.Entries[int(cqIdx)].Data == uint64(i+1) {
				exContent = append(exContent, readBuff...)
				fmt.Printf("got content: %+v\n", string(readBuff))
				*cq.Head += 1
				offset += uint64(cq.Entries[int(cqIdx)].Res)
			}
		}
	}

	if !bytes.Equal(content, exContent) {
		log.Fatalf("Expected: %+v, got: %+v", string(content), string(exContent))
	}
}
