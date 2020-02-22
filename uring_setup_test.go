// +build linux

package iouring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMmapSubmitRing(t *testing.T) {
	var p Params
	fd, err := Setup(1024, &p)
	require.NoError(t, err)
	var (
		cq CompletionQueue
		sq SubmitQueue
	)
	require.NoError(t, MmapRing(fd, &p, &sq, &cq))
}
