package iouring

import (
	"reflect"
	"runtime"
	"unsafe"
)

func saferStringToBytes(s *string) []byte {
	bytes := make([]byte, 0, 0)

	// Shameless stolen from:
	// See: https://github.com/jlauinger/go-safer
	// create the string and slice headers by casting. Obtain pointers to the
	// headers to be able to change the slice header properties in the next step
	stringHeader := (*reflect.StringHeader)(unsafe.Pointer(s))
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&bytes))

	// set the slice's length and capacity temporarily to zero (this is actually
	// unnecessary here because the slice is already initialized as zero, but if
	// you are reusing a different slice this is important
	sliceHeader.Len = 0
	sliceHeader.Cap = 0

	// change the slice header data address
	sliceHeader.Data = stringHeader.Data

	// set the slice capacity and length to the string length
	sliceHeader.Cap = stringHeader.Len
	sliceHeader.Len = stringHeader.Len

	// use the keep alive dummy function to make sure the original string s is not
	// freed up until this point
	runtime.KeepAlive(s)

	return bytes
}
