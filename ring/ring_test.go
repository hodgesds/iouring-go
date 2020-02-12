// +build linux

package ring

import (
	"testing"

	"github.com/hodgesds/iouring-go"
)

func TestNewRing(t *testing.T) {
	var p iouring.Params
	_, err := NewRing(2048, &p)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewRingInvalidSize(t *testing.T) {
	var p iouring.Params
	_, err := NewRing(99999, &p)
	if err == nil {
		t.Fatal("expected NewRing to return an error")
	}
}
