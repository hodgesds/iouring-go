// +build linux

package iouring

import (
	"net"
	"os"
	"runtime"
)

// ringShard is used to shard io operations across multiple rings.
type ringShard struct {
	rings []*ring
}

// NewSharded returns a Ring interface that is sharded across rings.
func NewSharded(size uint, p *Params, opts ...RingOption) (Ring, error) {
	numCPU := runtime.NumCPU()
	rs := &ringShard{
		rings: make([]*ring, numCPU),
	}

	for i := 0; i < numCPU; i++ {
		r, err := New(size, p, opts...)
		if err != nil {
			return nil, err
		}
		rs.rings[i] = r.(*ring)
	}

	return rs, nil
}

// CQ returns the CompletionQueue for the ring.
func (rs *ringShard) CQ() *CompletionQueue {
	return rs.rings[0].cq
}

// SQ returns the SubmitQueue for the ring.
func (rs *ringShard) SQ() *SubmitQueue {
	return rs.rings[0].sq
}

// EventFd returns the file descriptor of the eventfd if it is set, otherwise
// it returns the default value of -1.
func (rs *ringShard) EventFd() int {
	return rs.rings[0].eventFd
}

// Fd returns the file descriptor of the ring.
func (rs *ringShard) Fd() int {
	return rs.rings[0].fd
}

// FileRegistry returns the FileRegistry for the Ring.
func (rs *ringShard) FileRegistry() FileRegistry {
	return rs.rings[0].fileReg
}

// FileReadWriter returns an io.ReadWriter from an os.File that uses the ring.
// Note that is is not valid to use other operations on the file (Seek/Close)
// in combination with the reader.
func (rs *ringShard) FileReadWriter(f *os.File) (ReadWriteSeekerCloser, error) {
	return rs.rings[0].fileReadWriter(f)
}

func (rs *ringShard) SockoptListener(network, address string, errHandler func(error), sockopts ...int) (net.Listener, error) {
	return rs.rings[0].SockoptListener(network, address, errHandler, sockopts...)
}
func (rs *ringShard) Stop() error {
	for _, ring := range rs.rings {
		if err := ring.Stop(); err != nil {
			return err
		}
	}
	return nil
}

func (rs *ringShard) SubmitEntry() (*SubmitEntry, func()) {
	return rs.rings[0].SubmitEntry()
}
