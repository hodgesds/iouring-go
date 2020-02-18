// +build linux

package iouring

import (
	"testing"
)

func TestEnter(t *testing.T) {
	p := Params{}
	fd, err := Setup(1024, &p)
	if err != nil {
		t.Fatal(err)
	}
	var sq SubmitQueue
	if err := MmapSubmitRing(fd, &p, &sq); err != nil {
		t.Fatal(err)
	}
	var cq CompletionQueue
	if err := MmapCompletionRing(fd, &p, &cq); err != nil {
		t.Fatal(err)
	}
	if err := Enter(fd, 10, 10, 0, nil); err != nil {
		t.Fatal(err)
	}
}
