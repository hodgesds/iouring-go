// +build linux

package iouring

// RingOption is an option for configuring a Ring.
type RingOption func(*Ring) error

// WithFileRegistry is used to configure a FileRegistry for use with a Ring.
func WithFileRegistry(reg FileRegistry) RingOption {
	return func(r *Ring) error {
		r.fileReg = reg
		return nil
	}
}

// WithDebug is used to print additional debug information.
func WithDebug() RingOption {
	return func(r *Ring) error {
		r.debug = true
		return nil
	}
}
