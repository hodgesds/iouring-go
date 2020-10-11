// +build linux

package iouring

import (
	"encoding/binary"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var (
	errRingUnavailable = errors.New("ring unavailable")
)

// PrepareAccept is used to prepare a SQE for an accept(2) call.
func (r *Ring) PrepareAccept(
	fd int,
	addr syscall.Sockaddr,
	socklen uint32,
	flags int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Accept
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&addr)))
	sqe.Offset = uint64(socklen)
	sqe.UFlags = int32(flags)

	ready()
	return sqe.UserData, nil
}

// PrepareClose is used to prepare a close(2) call.
func (r *Ring) PrepareClose(fd int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}
	sqe.Opcode = Close
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)

	ready()
	return sqe.UserData, nil
}

// Close is implements close(2).
func (r *Ring) Close(fd int) error {
	id, err := r.PrepareClose(fd)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

// PrepareConnect is used to prepare a SQE for a connect(2) call.
func (r *Ring) PrepareConnect(
	fd int,
	addr syscall.Sockaddr,
	socklen uint32,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Connect
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&addr)))
	sqe.Len = socklen

	ready()
	return sqe.UserData, nil
}

// PrepareFadvise is used to prepare a fadvise call.
func (r *Ring) PrepareFadvise(
	fd int, offset uint64, n uint32, advise int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Fadvise
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Len = n
	sqe.Offset = offset
	sqe.UFlags = int32(advise)

	ready()
	return sqe.UserData, nil
}

// Fadvise implements fadvise.
func (r *Ring) Fadvise(fd int, offset uint64, n uint32, advise int) error {
	id, err := r.PrepareFadvise(fd, offset, n, advise)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

// PrepareFallocate is used to prepare a fallocate call.
func (r *Ring) PrepareFallocate(
	fd int, mode uint32, offset int64, n int64) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Fallocate
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = uint64(n)
	sqe.Len = mode
	sqe.Offset = uint64(offset)

	ready()
	return sqe.UserData, nil
}

// Fallocate implements fallocate.
func (r *Ring) Fallocate(fd int, mode uint32, offset int64, n int64) error {
	id, err := r.PrepareFallocate(fd, mode, offset, n)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

// PrepareFsync is used to prepare a fsync(2) call.
func (r *Ring) PrepareFsync(fd int, flags int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}
	sqe.Opcode = Fsync
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.UFlags = int32(flags)

	ready()
	return sqe.UserData, nil
}

// Fsync implements fsync(2).
func (r *Ring) Fsync(fd int, flags int) error {
	id, err := r.PrepareFsync(fd, flags)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

// PrepareNop is used to prep a nop.
func (r *Ring) PrepareNop() (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}
	sqe.Opcode = Nop
	sqe.UserData = r.ID()
	sqe.Fd = -1

	ready()
	return sqe.UserData, nil
}

// Nop is a nop.
func (r *Ring) Nop() error {
	id, err := r.PrepareNop()
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

// PollAdd is used to add a poll to a fd.
func (r *Ring) PollAdd(fd int, mask int) error {
	id, err := r.PreparePollAdd(fd, mask)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

// PreparePollAdd is used to prepare a SQE for adding a poll.
func (r *Ring) PreparePollAdd(fd int, mask int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}
	sqe.Opcode = PollAdd
	sqe.Fd = int32(fd)
	sqe.UFlags = int32(mask)
	sqe.UserData = r.ID()

	ready()
	return sqe.UserData, nil
}

// PrepareReadv is used to prepare a readv SQE.
func (r *Ring) PrepareReadv(
	fd int,
	iovecs []*syscall.Iovec,
	offset int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Readv
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&iovecs[0])))
	sqe.Len = uint32(len(iovecs))
	sqe.Offset = uint64(offset)

	ready()
	return sqe.UserData, nil
}

// PrepareRecvmsg is used to prepare a recvmsg SQE.
func (r *Ring) PrepareRecvmsg(
	fd int,
	msg *syscall.Msghdr,
	flags int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = RecvMsg
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(msg)))
	sqe.Len = 1
	sqe.Offset = 0
	sqe.UFlags = int32(flags)

	ready()
	return sqe.UserData, nil
}

// Splice implements splice using a ring.
func (r *Ring) Splice(
	inFd int,
	inOff *int64,
	outFd int,
	outOff *int64,
	n int,
	flags int,
) (int64, error) {
	id, err := r.PrepareSplice(inFd, inOff, outFd, outOff, n, flags)
	if err != nil {
		return 0, err
	}
	// TODO: replace complete with something more efficient.
	errno, res := r.complete(id)
	if errno < 0 {
		return 0, syscall.Errno(-errno)
	}
	runtime.KeepAlive(inOff)
	runtime.KeepAlive(outOff)
	return int64(res), nil
}

// PrepareSplice is used to prepare a SQE for a splice(2).
func (r *Ring) PrepareSplice(
	inFd int,
	inOff *int64,
	outFd int,
	outOff *int64,
	n int,
	flags int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Splice
	sqe.Fd = int32(outFd)
	if inOff != nil {
		sqe.Addr = uint64(uintptr(unsafe.Pointer(&inOff)))
	} else {
		sqe.Addr = 0
	}
	sqe.Len = uint32(n)
	if outOff != nil {
		sqe.Offset = uint64(uintptr(unsafe.Pointer(&outOff)))
	} else {
		sqe.Offset = 0
	}
	sqe.UFlags = int32(flags)
	// BUG: need to convert the inFd to the union member of the SQE
	anon := [24]byte{}
	binary.LittleEndian.PutUint32(anon[4:], uint32(inFd))
	sqe.Anon0 = anon
	sqe.UserData = r.ID()

	ready()
	return sqe.UserData, nil
}

// Statx implements statx using a ring.
func (r *Ring) Statx(
	dirfd int,
	path string,
	flags int,
	mask int,
	statx *unix.Statx_t,
) (err error) {
	id, err := r.PrepareStatx(dirfd, path, flags, mask, statx)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	// No GC until the request is done.
	runtime.KeepAlive(statx)
	runtime.KeepAlive(dirfd)
	runtime.KeepAlive(path)
	runtime.KeepAlive(mask)
	runtime.KeepAlive(flags)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

// PrepareStatx is used to prepare a Statx call and will return the request id
// (SQE UserData) of the SQE. After calling the returned callback function the
// ring is safe to be entered.
func (r *Ring) PrepareStatx(
	dirfd int,
	path string,
	flags int,
	mask int,
	statx *unix.Statx_t,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Statx
	sqe.Fd = int32(dirfd)
	if path != "" {
		// TODO: could probably avoid the conversion to []byte
		b := saferStringToBytes(&path)
		sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))
	}
	sqe.Len = uint32(mask)
	sqe.Offset = (uint64)(uintptr(unsafe.Pointer(statx)))
	sqe.UFlags = int32(flags)
	sqe.UserData = r.ID()

	ready()
	return sqe.UserData, nil
}

// PrepareTimeout is used to prepare a timeout SQE.
func (r *Ring) PrepareTimeout(
	ts *syscall.Timespec, count int, flags int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Timeout
	sqe.UserData = r.ID()
	sqe.UFlags = int32(flags)
	sqe.Fd = -1
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(ts)))
	sqe.Len = 1
	sqe.Offset = uint64(count)

	ready()
	return sqe.UserData, nil
}

// PrepareTimeoutRemove is used to prepare a timeout removal.
func (r *Ring) PrepareTimeoutRemove(data uint64, flags int) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = TimeoutRemove
	sqe.UserData = r.ID()
	sqe.UFlags = int32(flags)
	sqe.Fd = -1
	sqe.Addr = data
	sqe.Len = 0
	sqe.Offset = 0

	ready()
	return sqe.UserData, nil
}

// PrepareRead is used to prepare a read SQE.
func (r *Ring) PrepareRead(
	fd int,
	b []byte,
	offset uint64,
	flags uint8,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Read
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Len = uint32(len(b))
	sqe.Flags = flags
	sqe.Offset = offset
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	ready()
	return sqe.UserData, nil
}

// PrepareReadFixed is used to prepare a fixed read SQE.
func (r *Ring) PrepareReadFixed(
	fd int,
	b []byte,
	flags uint8,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = ReadFixed
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Len = uint32(len(b))
	sqe.Flags = flags
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	ready()
	return sqe.UserData, nil
}

// PrepareWrite is used to prepare a Write SQE.
func (r *Ring) PrepareWrite(
	fd int,
	b []byte,
	offset uint64,
	flags uint8,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Write
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Len = uint32(len(b))
	sqe.Flags = flags
	sqe.Offset = offset
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	ready()
	return sqe.UserData, nil
}

// PrepareWriteFixed is used to prepare a fixed write SQE.
func (r *Ring) PrepareWriteFixed(
	fd int,
	b []byte,
	flags uint8,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = WriteFixed
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Len = uint32(len(b))
	sqe.Flags = flags
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	ready()
	return sqe.UserData, nil
}

// PrepareWritev is used to prepare a writev SQE.
func (r *Ring) PrepareWritev(
	fd int,
	iovecs []*syscall.Iovec,
	offset int,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Writev
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&iovecs[0])))
	sqe.Len = uint32(len(iovecs))
	sqe.Offset = uint64(offset)

	ready()
	return sqe.UserData, nil
}

// PrepareSend is used to prepare a Send SQE.
func (r *Ring) PrepareSend(
	fd int,
	b []byte,
	flags uint8,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Send
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Len = uint32(len(b))
	sqe.Flags = flags
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	ready()
	return sqe.UserData, nil
}

// Send is used to send data to a socket.
func (r *Ring) Send(
	fd int,
	b []byte,
	flags uint8,
) error {
	id, err := r.PrepareSend(fd, b, flags)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	// No GC until the request is done.
	runtime.KeepAlive(b)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}

// PrepareRecv is used to prepare a Recv SQE.
func (r *Ring) PrepareRecv(
	fd int,
	b []byte,
	flags uint8,
) (uint64, error) {
	sqe, ready := r.SubmitEntry()
	if sqe == nil {
		return 0, errRingUnavailable
	}

	sqe.Opcode = Recv
	sqe.UserData = r.ID()
	sqe.Fd = int32(fd)
	sqe.Len = uint32(len(b))
	sqe.Flags = flags
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&b[0])))

	ready()
	return sqe.UserData, nil
}

// Recv is used to recv data on a socket.
func (r *Ring) Recv(
	fd int,
	b []byte,
	flags uint8,
) error {
	id, err := r.PrepareRecv(fd, b, flags)
	if err != nil {
		return err
	}
	errno, _ := r.complete(id)
	// No GC until the request is done.
	runtime.KeepAlive(b)
	if errno < 0 {
		return syscall.Errno(-errno)
	}
	return nil
}
