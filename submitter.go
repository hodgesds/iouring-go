// +build linux

package iouring

import "time"

type submitter interface {
	submit(uint64)
	stop()
}

type ringSubmitter struct {
	r        *Ring
	done     chan struct{}
	work     chan struct{}
	deadline time.Duration
}

func newRingSubmitter(r *Ring, deadline time.Duration) *ringSubmitter {
	return &ringSubmitter{
		r:        r,
		done:     make(chan struct{}),
		work:     make(chan struct{}, 128),
		deadline: deadline,
	}
}

func (s *ringSubmitter) submit(reqID uint64) {
	// We don't actually care about the request id.
	s.work <- struct{}{}
}

func (s *ringSubmitter) run() {
	timer := time.NewTimer(s.deadline)
	if !timer.Stop() {
		<-timer.C
	}
	count := 0
	seen := 0
	timerActive := false
	for {
		select {
		case <-timer.C:
		enter:
			n, err := s.r.Enter(uint(count), uint(0), EnterGetEvents, nil)
			if err != nil {
				continue
			}
			seen += n
			if seen < count {
				goto enter
			}
			seen = 0
			count = 0
			timerActive = false

		case <-s.work:
			if !timerActive {
				timerActive = true
				timer.Reset(s.deadline)
			}
			count++

		case <-s.done:
			if !timer.Stop() {
				<-timer.C
			}
			return
		}
	}
}

func (s *ringSubmitter) stop() {
	s.done <- struct{}{}
}
