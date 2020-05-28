// +build linux

package iouring

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
)

const (
	pollin = 0x0001

	// SOReuseport is the socket option to reuse socket port.
	SOReuseport int = 0x0F

	// TCPFastopen is the socket option to open a TCP fast open.
	TCPFastopen int = 0x17
)

// FastOpenAllowed return nil if fast open is enabled.
func FastOpenAllowed() error {
	b, err := ioutil.ReadFile("/proc/sys/net/ipv4/tcp_fack")
	if err != nil {
		return err
	}
	allowed, err := strconv.Atoi(strings.Replace(string(b), "\n", "", -1))
	if err != nil {
		return err
	}

	if allowed != 3 {
		return fmt.Errorf("set /proc/sys/net/ipv4/tcp_fastopen to 3")
	}

	return nil
}

type connInfo struct {
	fd     int
	id     uint64
	sqeIds chan uint64
}

type addr struct {
	net string
	s   string
}

// Network implements the net.Addr interface.
func (a *addr) Network() string {
	return a.net
}

// String implements the net.Addr interface.
func (a *addr) String() string {
	return a.s
}

type ringListener struct {
	debug      bool
	r          *Ring
	f          *os.File
	a          *addr
	stop       chan struct{}
	errHandler func(error)
	newConn    chan net.Conn
	connGet    chan chan net.Conn
}

// run is used to interact with the ring
func (l *ringListener) run() {
	id := l.r.ID()
	fd := int(l.f.Fd())
	cInfo := &connInfo{
		fd: fd,
		id: id,
	}
	sqe, commit := l.r.SubmitEntry()
	sqe.Opcode = PollAdd
	sqe.Fd = int32(fd)
	sqe.UFlags = int32(pollin)
	sqe.UserData = id
	commit()

	conns := map[uint64]*connInfo{id: cInfo}

	for {
		select {
		case <-l.stop:
			return
		default:
			_, err := l.r.Enter(1024, 1, EnterGetEvents, nil)
			if err != nil {
				if l.errHandler != nil {
					l.errHandler(err)
				}
				continue
			}
			l.walkCq(conns)
		}
	}
}

func (l *ringListener) walkCq(conns map[uint64]*connInfo) {
	head := atomic.LoadUint32(l.r.cq.Head)
	tail := atomic.LoadUint32(l.r.cq.Tail)
	mask := atomic.LoadUint32(l.r.cq.Mask)
	if head&mask == tail&mask {
		return
	}

	seenIdx := head & mask
	seenEnd := false
	if l.debug {
		sqHead := *l.r.sq.Head
		sqTail := *l.r.sq.Tail
		sqMask := *l.r.sq.Mask
		cqHead := *l.r.cq.Head
		cqTail := *l.r.cq.Tail
		cqMask := *l.r.cq.Mask
		fmt.Printf("sq head: %v tail: %v\nsq entries: %+v\n", sqHead&sqMask, sqTail&sqMask, l.r.sq.Entries[:9])
		fmt.Printf("cq head: %v tail: %v\ncq entries: %+v\n", cqHead&cqMask, cqTail&cqMask, l.r.cq.Entries[:9])
	}
	for i := seenIdx; i <= tail&mask; i++ {
		cqe := l.r.cq.Entries[i]
		if (cqe.Flags&CqSeenFlag == CqSeenFlag || cqe.IsZero()) && !seenEnd {
			seenIdx = i
		} else {
			seenEnd = true
		}
		cInfo, ok := conns[cqe.UserData]
		if !ok {
			continue
		}
		l.r.cq.Entries[i].Flags |= CqSeenFlag
		head = atomic.LoadUint32(l.r.cq.Head)
		if seenIdx > head {
			atomic.CompareAndSwapUint32(l.r.cq.Head, head, seenIdx)
		}
		l.onListen(conns, cInfo)
		return
	}

	// Handle wrapping.
	seenIdx = uint32(0)
	seenEnd = false
	tail = atomic.LoadUint32(l.r.cq.Tail)
	mask = atomic.LoadUint32(l.r.cq.Mask)
	for i := uint32(0); i <= tail&mask; i++ {
		cqe := l.r.cq.Entries[i]
		if (cqe.Flags&CqSeenFlag == CqSeenFlag || cqe.IsZero()) && !seenEnd {
			seenIdx = i
		} else {
			seenEnd = true
		}
		cInfo, ok := conns[cqe.UserData]
		if !ok {
			continue
		}
		l.r.cq.Entries[i].Flags |= CqSeenFlag
		head = atomic.LoadUint32(l.r.cq.Head)
		if seenIdx > head {
			atomic.CompareAndSwapUint32(l.r.cq.Head, head, seenIdx)
		}
		l.onListen(conns, cInfo)
		return
	}
}

// onListen is called when processing a cqe for a listening socket.
func (l *ringListener) onListen(conns map[uint64]*connInfo, cInfo *connInfo) {
	var (
		newConnInfo connInfo
		offset      int64
		rc          = ringConn{
			stop: make(chan struct{}, 2),
			poll: make(chan uint64, 64),
			r:    l.r,
		}
	)
	for {
		// Wait for a new connection to arrive and add it to the ring.
		newFd, sa, err := syscall.Accept4(cInfo.fd, syscall.SOCK_NONBLOCK)
		if err != nil {
			// TODO: Log this or something?
			panic(err.Error())
		}
		rc.fd = newFd
		rc.laddr = l.a
		rc.raddr = &addr{net: l.a.net}
		switch sockType := sa.(type) {
		case *syscall.SockaddrInet4:
			rc.raddr.s = fmt.Sprintf("%s:%d", sockType.Addr, sockType.Port)
		case *syscall.SockaddrInet6:
			rc.raddr.s = fmt.Sprintf("%s:%d", sockType.Addr, sockType.Port)
		case *syscall.SockaddrUnix:
			rc.raddr.s = sockType.Name
		}
		rc.offset = &offset
		break
	}

	// Add the new connection back to the ring.
	sqe, commit := l.r.SubmitEntry()
	sqe.Opcode = PollAdd
	sqe.Fd = int32(rc.fd)
	sqe.UFlags = int32(pollin)
	sqe.UserData = newConnInfo.id
	commit()
	ready := int32(1)
	rc.pollReady = &ready
	go rc.run()

	// Add the old connection back as well.
	sqe, commit = l.r.SubmitEntry()
	sqe.Opcode = PollAdd
	sqe.Fd = int32(cInfo.fd)
	sqe.UFlags = int32(pollin)
	sqe.UserData = uint64(cInfo.fd)
	commit()

	// Wait for the new connection to be accepted.
	// TODO: If this is unbuffered it will block, alternatively it could be
	// sent in a separate goroutine to ensure the main ring code isn't
	// blocking.
	l.newConn <- &rc
}

// Close implements the net.Listener interface.
func (l *ringListener) Close() error {
	close(l.stop)
	return l.f.Close()

}

// Addr implements the net.Listener interface.
func (l *ringListener) Addr() net.Addr {
	return l.a
}

// Accept implements the net.Listener interface.
func (l *ringListener) Accept() (net.Conn, error) {
	return <-l.newConn, nil
}

// SockoptListener returns a net.Listener that is Ring based.
func (r *Ring) SockoptListener(network, address string, errHandler func(error), sockopts ...int) (net.Listener, error) {
	var (
		err      error
		fd       int
		sockAddr syscall.Sockaddr
	)
	l := &ringListener{
		r:          r,
		a:          &addr{net: network},
		stop:       make(chan struct{}),
		newConn:    make(chan net.Conn, 1024),
		connGet:    make(chan chan net.Conn),
		errHandler: errHandler,
	}

	switch network {
	case "tcp", "tcp4":
		fd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
		if err != nil {
			return nil, fmt.Errorf("could not open socket")
		}
		netAddr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return nil, fmt.Errorf("could not open socket")
		}
		l.a.net = netAddr.Network()
		l.a.s = netAddr.String()

		var ipAddr [4]byte
		copy(ipAddr[:], netAddr.IP)
		sockAddr = &syscall.SockaddrInet4{
			Port: netAddr.Port,
			Addr: ipAddr,
		}
	case "tcp6":
		fd, err = syscall.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, 0)
		if err != nil {
			return nil, fmt.Errorf("could not open socket")
		}
		netAddr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return nil, fmt.Errorf("could not open socket")
		}
		l.a.net = netAddr.Network()
		l.a.s = netAddr.String()

		ipAddr := [16]byte{}
		copy(ipAddr[:], netAddr.IP)
		sockAddr = &syscall.SockaddrInet6{
			Port: netAddr.Port,
			Addr: ipAddr,
		}
	case "unix":
		fd, err = syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
		sockAddr = &syscall.SockaddrUnix{
			Name: address,
		}
	case "udp", "udp4":
		fd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
		if err != nil {
			return nil, fmt.Errorf("could not open socket")
		}
		netAddr, err := net.ResolveUDPAddr(network, address)
		if err != nil {
			return nil, fmt.Errorf("could not open socket")
		}
		ipAddr := [4]byte{}
		copy(ipAddr[:], netAddr.IP)
		sockAddr = &syscall.SockaddrInet4{
			Port: netAddr.Port,
			Addr: ipAddr,
		}
	case "udp6":
		fd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
		if err != nil {
			return nil, fmt.Errorf("could not open socket")
		}
		netAddr, err := net.ResolveUDPAddr(network, address)
		if err != nil {
			return nil, fmt.Errorf("could not open socket")
		}
		l.a.net = netAddr.Network()
		l.a.s = netAddr.String()

		ipAddr := [16]byte{}
		copy(ipAddr[:], netAddr.IP)
		sockAddr = &syscall.SockaddrInet6{
			Port: netAddr.Port,
			Addr: ipAddr,
		}
	default:
		return nil, fmt.Errorf("unknown network family: %s", network)
	}
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}

	for _, sockopt := range sockopts {
		if sockopt == SOReuseport {
			err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, sockopt, 1)
			if err != nil {
				syscall.Close(fd)
				return nil, err
			}
		} else if sockopt == TCPFastopen {
			if err := FastOpenAllowed(); err != nil {
				return nil, err
			}
			err = syscall.SetsockoptInt(fd, syscall.SOL_TCP, sockopt, 1)
			if err != nil {
				syscall.Close(fd)
				return nil, err
			}
		}
	}

	if err := syscall.Bind(fd, sockAddr); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err := syscall.Listen(fd, syscall.SOMAXCONN); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	f := os.NewFile(uintptr(fd), "l")
	l.f = f
	l.debug = r.debug
	go l.run()

	return l, nil
}
