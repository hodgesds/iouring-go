// +build linux

package iouring

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnter(t *testing.T) {
	p := Params{}
	fd, err := Setup(1024, &p)
	if err != nil {
		t.Fatal(err)
	}
	defer require.NoError(t, syscall.Close(fd))
}
