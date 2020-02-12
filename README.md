# `io_uring` Go 
This library adds support for [`io_uring`](https://kernel.dk/io_uring.pdf) for
Go. This library is similar to [liburing](https://github.com/axboe/liburing).

### General Steps
1) Create the `io_uring` buffers
2) Setup mmap for both ring buffers
3) Submit requests, this is done through another system call.

## Other References
https://cor3ntin.github.io/posts/iouring/
