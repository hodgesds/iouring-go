// +build linux

package iouring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupInvalidEntries(t *testing.T) {
	var p Params
	_, err := Setup(0, &p)
	require.Error(t, err)
	_, err = Setup(8192, &p)
	require.Error(t, err)
	_, err = Setup(9999, &p)
	require.Error(t, err)
}

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
