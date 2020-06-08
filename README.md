# `io_uring` Go
[![GoDoc](https://godoc.org/github.com/hodgesds/iouring-go?status.svg)](https://godoc.org/github.com/hodgesds/iouring-go)

**WORK IN PROGRESS** This library adds support for [`io_uring`](https://kernel.dk/io_uring.pdf) for
Go. This library is similar to [liburing](https://github.com/axboe/liburing).
If you want to contribute feel free to send PRs or emails, there's plenty of
things that need cleaned up.

# Interacting with the Submit/Completion Queues

## Design
The library is designed so that if you want to use your own implementation for
handling submissions/completions that everything is available for use.
Alternatively, there helper methods on the `Ring` struct that also interact
with standard library interfaces as well. There is also a interface for
creating a `net.Listener`, but it is still a work in progress.

## Submission Queue
The submission and completion queues are both mmap'd as slices, the question
then becomes how to design an efficient API that is also able to interact with
many of the standard library interfaces. One choice is to run a background
goroutine that manages all operations with the queues and use channels for
enqueuing requests. The downside of this approach is that are [outstanding
issues](https://github.com/golang/go/issues/8899) with the design of channels
may make it suboptimal for high throughput IO.

[`liburing`](https://github.com/axboe/liburing) uses memory barriers for
interacting appropriately with the submission/completion queues of `io_uring`.
One problem with the memory model of Go is that it uses [weak
atomics](https://github.com/golang/go/issues/5045) which can make it difficult
to use `sync/atomic` in all situations. If certain IO operations are to be
carriered out in a specific order then this becomes a real challenge.

The current challenge with the SQ is that currently for each reader/writer
interface every time the a read or write is submitted the
[`Enter`](https://godoc.org/github.com/hodgesds/iouring-go#Enter) method is
called on the ring. These could be batched (with a small latency penalty) and
allow for a single enter of the ring, which would result in fewer syscalls.

## Completion Queue
Completion queues have the difficulty of many concurrent readers which
need to synchronize updating the position of the head. The current solution
is to have a separate background goroutine that tracks the position of the
out of order completions and updates the head as necessary. This separates the
logic of synchronizing updating of the CQ head and handling a SQ request

# Setup
Ulimit values for locked memory address space may need to be adjusted. If the
following error occurs when running tests then the `memlock` value in
[`/etc/security/limits.conf`](https://linux.die.net/man/5/limits.conf) may need
to be increased.

```
=== RUN   TestNew
    TestNew: ring_test.go:13:
                Error Trace:    ring_test.go:13
                Error:          Received unexpected error:
                                cannot allocate memory
                Test:           TestNew
```

The ulimit value must be greater than the ring size, use `ulimit -l` to view
the current limit.


# Example
Here is a minimal example to get started that writes to a file using a ring:

```
package main

import (
	"log"
	"os"

	"github.com/hodgesds/iouring-go"
)

func main() {
	r, err := iouring.New(1024, &iouring.Params{
		Features: iouring.FeatNoDrop,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Open a file for registering with the ring.
	f, err := os.OpenFile("hello.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}

	// Register the file with the ring, which returns an io.WriteCloser.
	rw, err := r.FileReadWriter(f)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := rw.Write([]byte("hello io_uring!")); err != nil {
		log.Fatal(err)
	}

	// Close the WriteCloser, which closes the open file (f).
	if err := r.Close(); err != nil {
		log.Fatal(err)
	}
}
```

# Interacting with the SQ
The submission queue can be interacted with by using the
[`SubmitEntry`](https://godoc.org/github.com/hodgesds/iouring-go#Ring.SubmitEntry)
method on a `Ring`. The returned function **must** be called after all updates
to the `SubmitEntry` are complete and **before** the ring is entered. The
callback is used for synchronization across goroutines.

# Other References
https://cor3ntin.github.io/posts/iouring/

https://github.com/google/vectorio

https://github.com/shuveb/io_uring-by-example/blob/master/02_cat_uring/main.c

https://golang.org/pkg/syscall/#Iovec

https://github.com/golang/go/blob/master/src/runtime/mbarrier.go#L21
