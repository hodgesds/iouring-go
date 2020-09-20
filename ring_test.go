// +build linux

package iouring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	require.Nil(t, r.FileRegistry())
	require.NotNil(t, r.CQ())
	require.NotNil(t, r.SQ())

	require.NoError(t, r.Stop())
}

func TestNewRingInvalidSize(t *testing.T) {
	_, err := New(99999, nil)
	require.Error(t, err)
}
