#!/bin/bash

for fsize in 128 256 512 1024; do
	dd if=/dev/urandom of=test bs=1M count="$fsize"
	echo "benchmarking standard go file io"
	for i in {1..10}; do
		echo 3 > /proc/sys/vm/drop_caches
		time go run stdio.go test test.copy
		cmp --silent test test.copy || exit 1
		rm -f test.copy
	done
	sync

	echo "benchmarking io_uring go file io"
	for i in {1..10}; do
		echo 3 > /proc/sys/vm/drop_caches
		time go run iouring.go test test.copy
		cmp  test test.copy || exit 1
		rm -f $1.copy
	done
done
