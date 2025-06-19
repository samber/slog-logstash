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

// failingMockConn is a simple in-memory net.Conn for testing that always fails Write.
// It supports concurrent Write and can be closed.
type failingMockConn struct {
	out    bytes.Buffer
	closed bool
	mu     sync.Mutex
}

func newFailingMockConn() *failingMockConn {
	return &failingMockConn{}
}

func (m *failingMockConn) Read(p []byte) (int, error) { return 0, io.EOF }
func (m *failingMockConn) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return 0, errors.New("Write failed")
}
func (m *failingMockConn) Close() error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return nil
}
func (m *failingMockConn) LocalAddr() net.Addr                { return nil }
func (m *failingMockConn) RemoteAddr() net.Addr               { return nil }
func (m *failingMockConn) SetDeadline(t time.Time) error      { return nil }
func (m *failingMockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *failingMockConn) SetWriteDeadline(t time.Time) error { return nil }

// Test that if write fails, error is returned and following write attempt dials new conn
func TestReconnectConnWriteFails(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		var called bool
		conn := NewReconnectConn(func() (net.Conn, error) {
			if called {
				return newMockConn(), nil
			}
			called = true
			return newFailingMockConn(), nil
		}, 100*time.Millisecond)
		defer conn.Close() //nolint:errcheck

		n, err := conn.Write([]byte("aaaa"))

		if err == nil {
			t.Fatal("Expected error, got nil instead")
		}
		if n > 0 {
			t.Errorf("Expected no bytes to be written, got %d instead", n)
		}

		n, err = conn.Write([]byte("aaaa"))

		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != 4 {
			t.Errorf("Expected 4 bytes to be written, got %d instead", n)
		}
	})
}

// Test concurrent Write from multiple goroutines
func TestReconnectConnConcurrentWrite(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := NewReconnectConn(func() (net.Conn, error) {
			return newMockConn(), nil
		}, 100*time.Millisecond)
		defer conn.Close() //nolint:errcheck

		var wg sync.WaitGroup
		errCh := make(chan error, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msg := []byte("msg")
				for j := 0; j < 100; j++ {
					if _, err := conn.Write(msg); err != nil {
						errCh <- err
						return
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

// Test that if the conn runs out of retries, error is returned
func TestReconnectConnRetryLimitedRetryFail(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		var failCount int
		conn := NewReconnectConn(func() (net.Conn, error) {
			if failCount < 4 {
				failCount++
				return nil, errors.New("Could not connect")
			}
			return newMockConn(), nil
		}, 100*time.Millisecond)
		defer conn.Close() //nolint:errcheck
		conn.SetMaxRetries(2)

		n, err := conn.Write([]byte("aaaa"))

		if err == nil {
			t.Fatal("Expected error, got nil instead")
		}
		if n > 0 {
			t.Errorf("Expected no bytes to be written, got %d instead", n)
		}
	})
}

// Test that if dialer returns error, it is retried, with limited retries
func TestReconnectConnRetryDialerLimitedRetry(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		var failCount int
		conn := NewReconnectConn(func() (net.Conn, error) {
			if failCount < 2 {
				failCount++
				return nil, errors.New("Could not connect")
			}
			return newMockConn(), nil
		}, 100*time.Millisecond)
		defer conn.Close() //nolint:errcheck
		conn.SetMaxRetries(3)

		n, err := conn.Write([]byte("aaaa"))

		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != 4 {
			t.Errorf("Expected 4 bytes to be written, got %d instead", n)
		}
	})
}

// Test that if dialer returns error, it is retried with unlimited number of retries
func TestReconnectConnRetryDialer(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		var failCount int
		conn := NewReconnectConn(func() (net.Conn, error) {
			if failCount < 2 {
				failCount++
				return nil, errors.New("Could not connect")
			}
			return newMockConn(), nil
		}, 100*time.Millisecond)
		defer conn.Close() //nolint:errcheck

		n, err := conn.Write([]byte("aaaa"))

		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != 4 {
			t.Errorf("Expected 4 bytes to be written, got %d instead", n)
		}
	})
}

// Test that read fails, ReconnetcConn is write-only
func TestReconnectConnReadFails(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := NewReconnectConn(func() (net.Conn, error) {
			return newMockConn(), nil
		}, 5*time.Second)
		defer conn.Close() //nolint:errcheck

		n, err := conn.Read(make([]byte, 10))

		if err == nil {
			t.Error("Expected error, got nil instead")
		}
		if n > 0 {
			t.Errorf("Expected no bytes to be read, got %d instead", n)
		}
	})
}

// Test rapid repeated Close calls from multiple goroutines
func TestReconnectConnConcurrentRepeatedClose(t *testing.T) {
	testWithTimeout(t, func(t *testing.T) {
		conn := NewReconnectConn(func() (net.Conn, error) {
			return newMockConn(), nil
		}, 5*time.Second)

		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = conn.Close()
			}()
		}
		wg.Wait()
	})
}
