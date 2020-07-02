// +build linux

package iouring

import (
	"io"
	"os"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

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
	c       *completer
}

// getCqe is used for getting a CQE result and will retry up to one time.
func (i *ringFIO) getCqe(reqID uint64) (int, error) {
	if i.r.submitter != nil {
		i.r.submitter.submit(reqID)
	} else {
		_, err := i.r.Enter(uint(1), uint(1), EnterGetEvents, nil)
		if err != nil {
			return 0, err
		}
	}

	cq := i.r.cq
	foundIdx := 0
findCqe:
	head := atomic.LoadUint32(cq.Head)
	tail := atomic.LoadUint32(cq.Tail)
	mask := atomic.LoadUint32(cq.Mask)
	end := int(tail & mask)

	for x := int(head & mask); x < len(cq.Entries); x++ {
		cqe := cq.Entries[x]
		if cqe.UserData == reqID {
			if cqe.Res < 0 {
				return 0, syscall.Errno(-cqe.Res)
			}
			atomic.StoreInt64(i.fOffset, atomic.LoadInt64(i.fOffset)+int64(cqe.Res))
			foundIdx = x
			i.c.complete(foundIdx)
			return int(cqe.Res), nil
		}
		if x == end {
			goto findCqe
		}
	}
	tail = atomic.LoadUint32(cq.Tail)
	mask = atomic.LoadUint32(cq.Mask)
	end = int(tail & mask)
	for x := 0; x < end; x++ {
		cqe := cq.Entries[x]
		if cqe.UserData == reqID {
			if cqe.Res < 0 {
				return 0, syscall.Errno(-cqe.Res)
			}
			atomic.StoreInt64(i.fOffset, atomic.LoadInt64(i.fOffset)+int64(cqe.Res))
			foundIdx = x
			i.c.complete(foundIdx)
			return int(cqe.Res), nil
		}
		if x == end {
			goto findCqe
		}
	}
	goto findCqe
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

	return i.getCqe(reqID)
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

	n, err := i.getCqe(reqID)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return n, io.EOF
	}
	return n, nil
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

	return i.getCqe(reqID)
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

	n, err := i.getCqe(reqID)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return n, io.EOF
	}
	return n, nil
}

// Close implements the io.Closer interface.
func (i *ringFIO) Close() error {
	i.c.stop()
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
