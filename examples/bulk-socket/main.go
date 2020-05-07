package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
	"unsafe"

	gsse "github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/hodgesds/iouring-go"
	"github.com/r3labs/sse"
)

var (
	fds     = make([]int32, 0)
	message []byte
	ring    *iouring.Ring
)

func main() {
	msg := struct {
		ID      int
		Author  string
		Content string
	}{
		112,
		"Sample User",
		"Sample message 123",
	}

	// Create a static message to deliver that can also be compared against when we receive it over the network
	message, _ = json.Marshal(msg)

	ring, _ = iouring.New(1024, &iouring.Params{})

	// Start a new go routine that sends a message every second
	go sendMessage()

	r := gin.Default()

	// Create a SSE endpoint that hijacks all incoming connections and adds their underlying file descriptors to an array
	r.GET("/listen", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Writer.WriteHeaderNow()

		nc, _, _ := c.Writer.Hijack()

		sf, _ := nc.(*net.TCPConn).File()

		fds = append(fds, int32(sf.Fd()))
	})

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	addr := fmt.Sprintf("http://localhost:%d/listen", l.Addr().(*net.TCPAddr).Port)

	go http.Serve(l, r)

	// Spawn n many clients to establish an SSE
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		go spawnClient(addr)
	}

	select {}
}

type backOff struct{}

func (b *backOff) NextBackOff() time.Duration { return -1 }
func (b *backOff) Reset()                     {}

func spawnClient(addr string) {
	c := sse.NewClient(addr)
	c.ReconnectStrategy = &backOff{}

	// Subscribe to the SSE endpoint
	if err := c.Subscribe("", func(evt *sse.Event) {
		// If we receive an event that isn't equal to our preset message, it has been corrupted
		if string(message) != string(evt.Data) {
			log.Fatalf("Client received invalid response, expected: %s but got %s", string(message), string(evt.Data))
		}
	}); err != nil {
		log.Fatalln(err.Error())
	}
}

func sendMessage() {
	for {
		time.Sleep(1 * time.Second)

		if err := send(fds, message); err != nil {
			log.Fatal(err.Error())
		}
	}
}

func send(fds []int32, data []byte) error {
	fmt.Printf("Sending %d bytes to %d sockets\n", len(data), len(fds))

	var b bytes.Buffer
	// Encode the JSON message into an SSE
	if err := gsse.Encode(&b, gsse.Event{
		Event: "message",
		Data:  json.RawMessage(data),
	}); err != nil {
		return err
	}

	sdata := b.Bytes()

	wire := bytes.Buffer{}

	// Wrap the SSE into the chunked http wire format
	fmt.Fprintf(&wire, "%x\r\n", len(sdata))
	wire.Write(sdata)
	wire.WriteString("\r\n")

	rawData := wire.Bytes()

	addr := (uint64)(uintptr(unsafe.Pointer(&rawData[0])))
	length := uint32(len(rawData))

	// Queue up n many SQE's for each file descriptor
	for _, fd := range fds {
		e, commit := ring.SubmitEntry()

		e.Opcode = iouring.WriteFixed
		e.Fd = fd
		e.Addr = addr
		e.Len = length

		commit()
	}

	return ring.Enter(uint(len(fds)), uint(len(fds)), iouring.EnterGetEvents, nil)
}
