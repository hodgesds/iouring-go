// +build linux

package iouring

import (
	"sync/atomic"
)

type completer struct {
	cq     *CompletionQueue
	stopCh chan struct{}
	seen   chan int
}

func newCompleter(cq *CompletionQueue, bufSize int) *completer {
	return &completer{
		cq:     cq,
		stopCh: make(chan struct{}, 8),
		seen:   make(chan int, bufSize),
	}
}

func (c *completer) complete(id int) {
	c.seen <- id
}

func (c *completer) stop() {
	c.stopCh <- struct{}{}
}

func (c *completer) run() {
	unacked := map[int]struct{}{}
	for {
		select {
		case <-c.stopCh:
			return
		case id := <-c.seen:
			// TODO: is it bad to see twice?
			if _, ok := unacked[id]; !ok {
				unacked[id] = struct{}{}
			}
			head := atomic.LoadUint32(c.cq.Head)
			mask := atomic.LoadUint32(c.cq.Mask)
			seen := int(0)
			// Continue to move the head until the next value
			// hasn't arrived yet.
			curHead := int(head & mask)
			for {
				_, ok := unacked[curHead+seen]
				if !ok {
					break
				}
				delete(unacked, curHead+seen)
				seen++
			}
			atomic.AddUint32(c.cq.Head, uint32(seen))
		}
	}
}
