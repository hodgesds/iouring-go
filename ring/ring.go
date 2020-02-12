package ring

import (
	"github.com/hodgesds/iouring-go"
	"golang.org/x/sys/unix"
)

type ring struct {
	fd int
	cq *iouring.CompletionQueue
	sq *iouring.SubmitQueue
}

// NewRing is used to create and enter a Ring.
func NewRing(size uint, p *iouring.Params) (iouring.Ring, error) {
	fd, err := iouring.Setup(size, p)
	if err != nil {
		return nil, err
	}
	var sq iouring.SubmitQueue
	if err := iouring.MmapSubmitRing(fd, p, &sq); err != nil {
		return nil, err
	}
	var cq iouring.CompletionQueue
	if err := iouring.MmapCompletionRing(fd, p, &cq); err != nil {
		return nil, err
	}

	return &ring{
		fd: fd,
		cq: &cq,
		sq: &sq,
	}, nil
}

func (r *ring) Submit(toSubmit uint, minComplete uint, flags uint, sigset *unix.Sigset_t) error {
	// XXX: Make these options.
	if err := iouring.Enter(r.fd, toSubmit, minComplete, iouring.EnterGetevents, nil); err != nil {
		return err
	}
	return nil
}
