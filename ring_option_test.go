// +build linux

package iouring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithDebug(t *testing.T) {
	r, err := New(2048, nil, WithDebug())
	require.NoError(t, err)
	require.NotNil(t, r)
	require.True(t, r.debug)
}

func TestWithFileRegistry(t *testing.T) {
	r, err := New(2048, nil, WithFileRegistry())
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NotNil(t, r.FileRegistry())
}

func TestWithEventFd(t *testing.T) {
	r, err := New(2048, nil, WithEventFd(0, 0, false))
	require.NoError(t, err)
	require.NotNil(t, r)
	require.True(t, r.EventFd() > 0)

	r, err = New(2048, nil, WithEventFd(0, 0, true))
	require.NoError(t, err)
	require.NotNil(t, r)
	require.True(t, r.EventFd() > 0)
}
