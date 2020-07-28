# `io_uring` Go
[![GoDoc](https://godoc.org/github.com/hodgesds/iouring-go?status.svg)](https://godoc.org/github.com/hodgesds/iouring-go)

**WORK IN PROGRESS** This library adds support for [`io_uring`](https://kernel.dk/io_uring.pdf) for
Go. This library is similar to [liburing](https://github.com/axboe/liburing).
If you want to contribute feel free to send PRs or emails, there's plenty of
things that need cleaned up.

# ***SAFETY WARNING***
This library is unsafe to use in production. It violates some aspects of the Go
memory model and comes with no guarantees of not introducing security issues
and generally crashing your programs. It is an experimental exercise to see
what is possible with the Go runtime. However, it probably is not a good idea
to add `io_uring` to the runtime as it has many configuration options that are
useful to the end user.


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

# Benchmarks
I haven't really wanted to add any benchmarks as I haven't spent the time to
really write good benchmarks. However, here's some initial numbers with some
comments:

```
BenchmarkFileWrite
BenchmarkFileWrite/os-file-write-128
BenchmarkFileWrite/os-file-write-128-8                    245845              4649 ns/op          27.53 MB/s           0 B/op          0 allocs/op
BenchmarkFileWrite/os-file-write-512
BenchmarkFileWrite/os-file-write-512-8                    243472              4867 ns/op         105.20 MB/s           0 B/op          0 allocs/op
BenchmarkFileWrite/os-file-write-1024
BenchmarkFileWrite/os-file-write-1024-8                   212593              5320 ns/op         192.48 MB/s           0 B/op          0 allocs/op
BenchmarkFileWrite/os-file-write-2048
BenchmarkFileWrite/os-file-write-2048-8                   183775              6047 ns/op         338.69 MB/s           0 B/op          0 allocs/op
BenchmarkFileWrite/os-file-write-4096
BenchmarkFileWrite/os-file-write-4096-8                   143608              7614 ns/op         537.98 MB/s           0 B/op          0 allocs/op
BenchmarkRingWrite
BenchmarkRingWrite/ring-1024-write-128
BenchmarkRingWrite/ring-1024-write-128-8                  126456              9346 ns/op          13.70 MB/s          32 B/op          1 allocs/op
BenchmarkRingWrite/ring-1024-write-512
BenchmarkRingWrite/ring-1024-write-512-8                  119118             10702 ns/op          47.84 MB/s          32 B/op          1 allocs/op
BenchmarkRingWrite/ring-1024-write-1024
BenchmarkRingWrite/ring-1024-write-1024-8                 115423             10600 ns/op          96.60 MB/s          32 B/op          1 allocs/op
BenchmarkRingWrite/ring-8192-write-2048
BenchmarkRingWrite/ring-8192-write-2048-8                 103276             11006 ns/op         186.07 MB/s          32 B/op          1 allocs/op
BenchmarkRingWrite/ring-8192-write-4096
BenchmarkRingWrite/ring-8192-write-4096-8                  87127             13704 ns/op         298.90 MB/s          32 B/op          1 allocs/op
BenchmarkRingDeadlineWrite
BenchmarkRingDeadlineWrite/ring-1024-deadline-1ms-128
BenchmarkRingDeadlineWrite/ring-1024-deadline-1ms-128-8                   102620              9979 ns/op          12.83 MB/s          32 B/op          1 allocs/op
BenchmarkRingDeadlineWrite/ring-1024-deadline-100µs-512
BenchmarkRingDeadlineWrite/ring-1024-deadline-100µs-512-8                 118021             10479 ns/op          48.86 MB/s          32 B/op          1 allocs/op
BenchmarkRingDeadlineWrite/ring-1024-deadline-10µs-1024
BenchmarkRingDeadlineWrite/ring-1024-deadline-10µs-1024-8                 103600             10232 ns/op         100.08 MB/s          32 B/op          1 allocs/op
BenchmarkRingDeadlineWrite/ring-8192-deadline-1µs-2048
BenchmarkRingDeadlineWrite/ring-8192-deadline-1µs-2048-8                  101726             11330 ns/op         180.75 MB/s          32 B/op          1 allocs/op
BenchmarkRingDeadlineWrite/ring-8192-deadline-1µs-4096
BenchmarkRingDeadlineWrite/ring-8192-deadline-1µs-4096-8                   87483             13547 ns/op         302.35 MB/s          32 B/op          1 allocs/op
BenchmarkRingMultiWrite
    BenchmarkRingMultiWrite: ring_benchmark_test.go:207: 
```

The first benchmark is just regualar `os.File` `Write` calls. This benchmark
was run on Xeon E3-1505M v5 running on a luks encrypted consumer NVMe drive.
The first thing to note is that the ns/op for for increasing write sizes scales
from 4-8k. That seems pretty reasonable because the runtime is taking care of
handling the system call.

The `BenchmarkRingWrite` is roughly the same type of
benchmark with an `Enter` being called for each SQE (essentially 1 syscall per
write request). Note, that the ns/op is much higher because of all extra
"stuff" the ring is handling. It also has a single allocation because it uses a
monotonically increasing request id for tracking submissions with completions
(using the user data field in the SQE). The other thing to note is the ring
currently isn't using an eventfd for handling completions, it is doing the good
old fashion brute force approach of submitting the request and then aggressively
checking the CQ for the completion event. This is rather ineficient and burns
some CPU cycles. Switching to an eventfd approach would probably be the ideal
way to solve this problem. So the numbers showing roughly double the ns/op are
pretty reasonable given the current design, which explains the lower throughput
when doing a '1:1' comparison with Go file IO.

The `BenchmarkRingDeadlineWrite` is kind of similar to the `BenchmarkRingWrite`
only it uses a deadline approach for submissions. This in theory should handle
concurrent writes far better, but there is no benchmark that is using
concurrent writes as it is not the easiest type benchmark to write.

The multiwrite API is still a WIP and it in theory should allow for "fan out"
style writes to multiple FDs.

Note, this library is still usable to a point where you can come up with your
own concurrent io scheduling based on whatever huerestics you want (limiting IO
requests per user?!?!). Implementing the perfect IO scheduler for Go is not
really a goal of this project so this library will most likely have some
tradeoffs (ie. my spare time) when it comes to optimal scheduling algorithms.
If you are interested in this area feel free to send any PRs.

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
