// +build linux

package iouring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	r, err := New(2048)
	require.NoError(t, err)
	require.NotNil(t, r)

	require.NotZero(t, r.sq.Size)
	require.NotNil(t, r.sq.Head)
	require.NotNil(t, r.sq.Tail)
	require.NotNil(t, r.sq.Mask)
	require.NotNil(t, r.sq.Entries)
	require.NotNil(t, r.sq.Flags)
	require.NotNil(t, r.sq.Dropped)
	require.NotNil(t, r.sq.Entries)

	require.NotZero(t, r.cq.Size)
	require.NotNil(t, r.cq.Head)
	require.NotNil(t, r.cq.Tail)
	require.NotNil(t, r.cq.Mask)
	require.NotNil(t, r.cq.Entries)

	require.NoError(t, r.Close())
}

func TestNewRingInvalidSize(t *testing.T) {
	_, err := New(99999)
	require.Error(t, err)
}

func TestRingEnter(t *testing.T) {
	r, err := New(2048)
	require.NoError(t, err)
	require.NotNil(t, r)
	count := 0
	for i := r.SubmitHead(); i < r.SubmitTail(); i++ {
		r.sq.Entries[i] = SubmitEntry{
			Opcode:   Nop,
			UserData: uint64(i),
		}
		count++
	}
	err = r.Enter(1, uint(count), uint(count), EnterGetEvents, nil)
	require.NoError(t, err)
}
