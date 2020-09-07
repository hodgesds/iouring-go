// +build linux

package iouring

import (
	"io"
	"os"
	"runtime"
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
	fd      int32
	fOffset *int64
	c       *completer
}

// getCqe is used for getting a CQE result and will retry up to one time.
func (i *ringFIO) getCqe(reqID uint64, count, min int) (int, error) {
	// TODO: consider adding the submitter interface here, or move out the
	// submit function from this method all together.
	if count > 0 || min > 0 {
		_, err := i.r.Enter(uint(count), uint(min), EnterGetEvents, nil)
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
func (i *ringFIO) Write(b []byte) (int, error) {
	id, ready, err := i.PrepareWrite(b, 0)
	if err != nil {
		return 0, err
	}
	ready()
	n, err := i.getCqe(id, 1, 1)
	runtime.KeepAlive(b)
	return n, err
}

// PrepareWrite is used to prepare a Write SQE. The ring is able to be entered
// after the returned callback is called.
func (i *ringFIO) PrepareWrite(b []byte, flags uint8) (uint64, func(), error) {
	sqe, ready := i.r.SubmitEntry()
	if sqe == nil {
		return 0, nil, errRingUnavailable
	}

	sqe.Opcode = Write
	sqe.UserData = i.r.ID()
	sqe.Fd = i.fd
	sqe.Len = uint32(len(b))
	sqe.Flags = flags
	sqe.Offset = uint64(atomic.LoadInt64(i.fOffset))
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	return sqe.UserData, ready, nil
}

// PrepareRead is used to prepare a Read SQE. The ring is able to be entered
// after the returned callback is called.
func (i *ringFIO) PrepareRead(b []byte, flags uint8) (uint64, func(), error) {
	sqe, ready := i.r.SubmitEntry()
	if sqe == nil {
		return 0, nil, errRingUnavailable
	}

	sqe.Opcode = Read
	sqe.UserData = i.r.ID()
	sqe.Fd = i.fd
	sqe.Len = uint32(len(b))
	sqe.Flags = flags
	sqe.Offset = uint64(atomic.LoadInt64(i.fOffset))
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	return sqe.UserData, ready, nil
}

// Read implements the io.Reader interface.
func (i *ringFIO) Read(b []byte) (int, error) {
	id, ready, err := i.PrepareRead(b, 0)
	if err != nil {
		return 0, err
	}
	ready()
	n, err := i.getCqe(id, 1, 1)
	runtime.KeepAlive(b)
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
		return 0, errRingUnavailable
	}

	sqe.Opcode = Write
	sqe.UserData = i.r.ID()
	sqe.Fd = i.fd
	sqe.Len = uint32(len(b))
	sqe.Flags = 0
	sqe.Offset = uint64(o)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	ready()

	n, err := i.getCqe(sqe.UserData, 1, 1)
	runtime.KeepAlive(b)
	return n, err
}

// ReadAt implements the io.ReaderAt interface.
func (i *ringFIO) ReadAt(b []byte, o int64) (int, error) {
	sqe, ready := i.r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Read
	sqe.UserData = i.r.ID()
	sqe.Fd = i.fd
	sqe.Len = uint32(len(b))
	sqe.Flags = 0
	sqe.Offset = uint64(o)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	ready()

	n, err := i.getCqe(sqe.UserData, 1, 1)
	runtime.KeepAlive(b)
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
	id, err := i.r.PrepareClose(int(i.fd))
	if err != nil {
		return err
	}
	_, err = i.getCqe(id, 1, 1)
	if err != nil {
		return err
	}
	return nil
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
