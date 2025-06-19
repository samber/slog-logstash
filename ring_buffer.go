package sloglogstash

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
)

// NewRingBuffer creates a new RingBuffer with given buffer capacities (in bytes).
// The write buffer is message-based ([][]byte).
// The returned connection is safe for concurrent use and will asynchronously buffer writes.
func NewRingBuffer(conn net.Conn, writeCapBytes int) *RingBuffer {
	rb := &RingBuffer{
		Conn:          conn,
		writeBuffer:   make([][]byte, 0),
		writeCapBytes: writeCapBytes,
	}
	rb.writeCond = sync.NewCond(&rb.writeMu)
	rb.wg.Add(1)
	go rb.writeLoop()
	return rb
}

// RingBuffer wraps a net.Conn with an asynchronous, message-based write buffer.
//
// Writes are buffered and sent to the underlying connection in the background.
// If the buffer is full, the oldest messages are dropped to make space for new ones.
// The connection is safe for concurrent use by multiple goroutines.
type RingBuffer struct {
	net.Conn // Underlying network connection

	writeMu       sync.Mutex // Protects writeBuffer and writeLenBytes
	writeBuffer   [][]byte   // Slice of buffered write messages (message granularity)
	writeCapBytes int        // Total write buffer capacity in bytes
	writeLenBytes int        // Current total bytes in write buffer
	writeCond     *sync.Cond // Condition variable for blocking writeLoop

	dropped atomic.Uint64

	closing   atomic.Bool    // Indicates if the connection is closing (atomic)
	closed    atomic.Bool    // Indicates if the connection is closed (atomic)
	closeOnce sync.Once      // Ensures close is only performed once
	wg        sync.WaitGroup // WaitGroup for background goroutines
}

// Write implements io.Writer. It buffers the entire write or drops it if the buffer is full.
// If the buffer is full, it drops as many oldest messages as needed to make space for the new message.
// If the message is too large to ever fit, it is dropped.
func (rb *RingBuffer) Write(p []byte) (int, error) {
	if rb.closing.Load() || rb.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	msgLen := len(p)
	if msgLen == 0 {
		return 0, nil
	} else if msgLen > rb.writeCapBytes {
		// Message too large to ever fit
		return 0, nil
	}

	msgCopy := make([]byte, msgLen)
	copy(msgCopy, p)

	rb.writeMu.Lock()
	defer rb.writeMu.Unlock()

	// Drop as many oldest messages as needed in one go
	dropIdx := 0
	total := rb.writeLenBytes
	for dropIdx < len(rb.writeBuffer) && total+msgLen > rb.writeCapBytes {
		total -= len(rb.writeBuffer[dropIdx])
		dropIdx++
	}
	if dropIdx > 0 {
		rb.writeBuffer = rb.writeBuffer[dropIdx:]
		rb.writeLenBytes = total
		rb.dropped.Add(uint64(dropIdx))
	}

	rb.writeBuffer = append(rb.writeBuffer, msgCopy)
	rb.writeLenBytes += msgLen
	rb.writeCond.Signal()
	return msgLen, nil
}

// Close closes the connection and all background goroutines. It is safe to call multiple times.
// After Close, further Write or Flush calls will return io.ErrClosedPipe.
func (rb *RingBuffer) Close() error {
	var err error
	rb.closeOnce.Do(func() {
		rb.closing.Store(true)
		rb.writeCond.Broadcast()
		rb.wg.Wait()
		err = rb.Flush()
		if err != nil {
			return
		}
		rb.closed.Store(true)
		err = rb.Conn.Close()
		rb.writeMu.Lock()
		rb.writeBuffer = [][]byte{} // flush
		rb.writeMu.Unlock()
	})
	return err
}

// writeLoop runs in a goroutine and flushes the write buffer to the connection.
// It periodically checks the buffer and writes messages if available, exiting when closed.
func (rb *RingBuffer) writeLoop() {
	defer rb.wg.Done()
	for {
		rb.writeMu.Lock()
		for len(rb.writeBuffer) == 0 && !rb.closing.Load() {
			rb.writeCond.Wait()
		}
		if rb.closing.Load() {
			rb.writeMu.Unlock()
			return
		}
		backupDropped := rb.dropped.Load()
		msg := rb.writeBuffer[0]
		rb.writeLenBytes -= len(rb.writeBuffer[0])
		rb.writeBuffer = rb.writeBuffer[1:]
		rb.writeMu.Unlock()

		// Write outside lock
		_, err := rb.Conn.Write(msg)
		if err != nil {
			// In case of error, we need to put the message back to the buffer, but
			// only if it wasn't dropped.
			rb.writeMu.Lock()
			if rb.dropped.Load()-backupDropped == 0 {
				rb.writeLenBytes += len(msg)
				rb.writeBuffer = append([][]byte{msg}, rb.writeBuffer...)
			}
			rb.writeMu.Unlock()
		}
	}
}

// Flush writes all buffered messages to the connection.
// It blocks until all buffered messages are sent or an error occurs.
// This method is safe for concurrent use.
func (rb *RingBuffer) Flush() error {
	rb.writeMu.Lock()
	defer rb.writeMu.Unlock()

	if rb.closed.Load() {
		return io.ErrClosedPipe
	}

	for {
		if len(rb.writeBuffer) == 0 {
			return nil
		}

		_, err := rb.Conn.Write(rb.writeBuffer[0])
		if err != nil {
			return err
		}

		rb.writeLenBytes -= len(rb.writeBuffer[0])
		rb.writeBuffer = rb.writeBuffer[1:]
	}
}
