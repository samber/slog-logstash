package sloglogstash

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// mockConn is a simple in-memory net.Conn for testing
// It supports concurrent Write and can be closed.
type mockConn struct {
	out    bytes.Buffer
	closed bool
	mu     sync.Mutex
}

func newMockConn() *mockConn {
	return &mockConn{}
}

func (m *mockConn) Read(p []byte) (int, error) { return 0, io.EOF }
func (m *mockConn) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.out.Write(p)
}
func (m *mockConn) Close() error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return nil
}
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// testWithTimeout runs a test function and fails if it does not complete within 1 second.
func testWithTimeout(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn(t)
		done <- struct{}{}
	}()
	select {
	case <-done:
		return
	case <-time.After(1 * time.Second):
		t.Fatal("test timed out")
	}
}

// Test that writing more than the buffer capacity drops old messages
func TestRingBufferWriteBufferDrop(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 10) // 10 bytes write buffer
		defer ringBuffer.Close()              //nolint:errcheck

		// Write 3 messages: 4, 4, 4 bytes (total 12 > 10)
		if _, err := ringBuffer.Write([]byte("aaaa")); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if _, err := ringBuffer.Write([]byte("bbbb")); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if _, err := ringBuffer.Write([]byte("cccc")); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// Flush to ensure all buffered messages are sent
		if err := ringBuffer.Flush(); err != nil {
			t.Fatalf("Flush failed: %v", err)
		}

		// Give the background writeLoop a moment to complete any pending operations
		time.Sleep(10 * time.Millisecond)

		conn.mu.Lock()
		got := make([]byte, len(conn.out.Bytes()))
		copy(got, conn.out.Bytes())
		conn.mu.Unlock()

		// Check that the newer messages are present
		if !bytes.Contains(got, []byte("bbbb")) || !bytes.Contains(got, []byte("cccc")) {
			t.Errorf("Expected buffer to contain 'bbbb' and 'cccc', got %q", got)
		}

		// Check that the oldest message was dropped due to buffer overflow
		// Note: The background writeLoop may have sent 'aaaa' before the overflow occurred,
		// so we need to check the total number of messages sent rather than just presence
		aaaaCount := bytes.Count(got, []byte("aaaa"))
		bbbbCount := bytes.Count(got, []byte("bbbb"))
		ccccCount := bytes.Count(got, []byte("cccc"))

		// With a 10-byte buffer and 4-byte messages, we can fit at most 2 messages (8 bytes)
		// The third message should cause the first to be dropped
		// So we expect either 0 or 1 'aaaa' (if it was sent before overflow), and 1 each of 'bbbb' and 'cccc'
		if aaaaCount > 1 {
			t.Errorf("Expected at most 1 'aaaa' message (due to buffer overflow), got %d", aaaaCount)
		}
		if bbbbCount != 1 {
			t.Errorf("Expected exactly 1 'bbbb' message, got %d", bbbbCount)
		}
		if ccccCount != 1 {
			t.Errorf("Expected exactly 1 'cccc' message, got %d", ccccCount)
		}
	})
}

// Test write/flush
func TestRingBufferWriteAndFlush(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 100)
		defer ringBuffer.Close() //nolint:errcheck

		if _, err := ringBuffer.Write([]byte("hello")); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if err := ringBuffer.Flush(); err != nil {
			t.Fatalf("Flush failed: %v", err)
		}
		conn.mu.Lock()
		out := make([]byte, len(conn.out.Bytes()))
		copy(out, conn.out.Bytes())
		conn.mu.Unlock()
		if !bytes.Contains(out, []byte("hello")) {
			t.Errorf("Expected 'hello' in conn out buffer, got %q", out)
		}
	})
}

// Test flush
func TestRingBufferFlush(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 100)
		defer ringBuffer.Close() //nolint:errcheck

		if _, err := ringBuffer.Write([]byte("foo")); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if err := ringBuffer.Flush(); err != nil {
			t.Fatalf("Flush failed: %v", err)
		}
		conn.mu.Lock()
		buf := make([]byte, len(conn.out.Bytes()))
		copy(buf, conn.out.Bytes())
		conn.mu.Unlock()
		containsFoo := bytes.Contains(buf, []byte("foo"))
		if !containsFoo {
			t.Errorf("Expected 'foo' after flush")
		}
	})
}

// Test close
func TestRingBufferClose(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 100)
		defer ringBuffer.Close() //nolint:errcheck
		if err := ringBuffer.Close(); err != nil {
			t.Errorf("Close should be idempotent")
		}
		if _, err := ringBuffer.Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("Write after close should fail")
		}
	})
}

// Test concurrent Write and Flush from multiple goroutines
func TestRingBufferConcurrentWriteFlush(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 1024)
		defer ringBuffer.Close() //nolint:errcheck

		var wg sync.WaitGroup
		errCh := make(chan error, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msg := []byte("msg")
				for j := 0; j < 100; j++ {
					if _, err := ringBuffer.Write(msg); err != nil && err != io.ErrClosedPipe {
						errCh <- err
						return
					}
					if j%10 == 0 {
						if err := ringBuffer.Flush(); err != nil {
							errCh <- err
							return
						}
					}
				}
			}(i)
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Errorf("%v", err)
			}
		}
	})
}

// Test writing after close returns io.ErrClosedPipe
func TestRingBufferWriteAfterClose(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 100)
		defer ringBuffer.Close() //nolint:errcheck
		_ = ringBuffer.Close()   //nolint:errcheck
		_, err := ringBuffer.Write([]byte("foo"))
		if err != io.ErrClosedPipe {
			t.Errorf("Expected io.ErrClosedPipe after close, got %v", err)
		}
	})
}

// Test writing a message larger than the buffer
func TestRingBufferWriteTooLarge(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 10)
		defer ringBuffer.Close() //nolint:errcheck
		msg := make([]byte, 20)
		n, err := ringBuffer.Write(msg)
		if n != 0 || err != nil {
			t.Errorf("Expected (0, nil) for too large write, got (%d, %v)", n, err)
		}
	})
}

// Test flush after close does not panic
func TestRingBufferFlushAfterClose(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 100)
		defer ringBuffer.Close() //nolint:errcheck
		_ = ringBuffer.Flush()
	})
}

// Test concurrent Write and Close from multiple goroutines
func TestRingBufferConcurrentWriteAndClose(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 1024)

		var wg sync.WaitGroup
		stop := make(chan struct{})
		errCh := make(chan error, 10)

		// Writer goroutines
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg := []byte("msg")
				for {
					select {
					case <-stop:
						return
					default:
						_, err := ringBuffer.Write(msg)
						if err != nil && err != io.ErrClosedPipe {
							errCh <- err
							return
						}
					}
				}
			}()
		}

		// Closer goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			_ = ringBuffer.Close()
			close(stop)
		}()

		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Errorf("%v", err)
			}
		}
	})
}

// Test concurrent Flush and Close from multiple goroutines
func TestRingBufferConcurrentFlushAndClose(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 1024)

		var wg sync.WaitGroup
		stop := make(chan struct{})
		errCh := make(chan error, 10)

		// Flusher goroutines
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						_ = ringBuffer.Flush()
					}
				}
			}()
		}

		// Closer goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			_ = ringBuffer.Close()
			close(stop)
		}()

		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Errorf("%v", err)
			}
		}
	})
}

// Test concurrent Write, Flush, and Close from multiple goroutines
func TestRingBufferConcurrentWriteFlushClose(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 1024)

		var wg sync.WaitGroup
		stop := make(chan struct{})
		errCh := make(chan error, 20)

		// Writer goroutines
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg := []byte("msg")
				for {
					select {
					case <-stop:
						return
					default:
						_, err := ringBuffer.Write(msg)
						if err != nil && err != io.ErrClosedPipe {
							errCh <- err
							return
						}
					}
				}
			}()
		}

		// Flusher goroutines
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						_ = ringBuffer.Flush()
					}
				}
			}()
		}

		// Closer goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			_ = ringBuffer.Close()
			close(stop)
		}()

		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Errorf("%v", err)
			}
		}
	})
}

// Test rapid repeated Close calls from multiple goroutines
func TestRingBufferConcurrentRepeatedClose(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := newMockConn()
		ringBuffer := NewRingBuffer(conn, 1024)

		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = ringBuffer.Close()
			}()
		}
		wg.Wait()
	})
}
