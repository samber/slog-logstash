package sloglogstash

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var _ net.Conn = (*ReconnectConn)(nil)

// ReconnectConn wraps a net.Conn with reconnects, but it only supports writing.
//
// It attempts to dial connection using specified dialer function.
// Dialer function is retried in case of failures until success or until maximum
// number of attemps is reached (unlimited by default).
//
// In case any error occurs when writing to the underlying connection, the current
// connection is closed and error is returned. A following write attempt dials
// a new connection (with retries).
//
// The connection is safe for concurrent use by multiple goroutines.
type ReconnectConn struct {
	dialer        func() (net.Conn, error) // Function to dial new connection
	conn          net.Conn                 // Current active connection
	mu            sync.Mutex               // Mutex to protect conn
	writeDeadline time.Time                // Deadline for future write calls

	retryDelay      time.Duration // Delay between reconnect attempts
	maxConnAttempts int           // Maximum number of connection attempts, infinite if set to 0 (the default)

	closing   atomic.Bool // Indicates if the connection is closing/closed (atomic)
	closeOnce sync.Once   // Ensures close is only performed once
}

// NewReconnectConn creates a new ReconnectConn with given dialer function and retry delay.
// The dialer function should dial a new net.Conn or return error in case of failure.
// The returned connection is safe for concurrent use and will attempt to dial a new connection in case of failures.
func NewReconnectConn(dialer func() (net.Conn, error), retryDelay time.Duration) *ReconnectConn {
	return &ReconnectConn{
		dialer: dialer,
		mu:     sync.Mutex{},

		retryDelay: retryDelay,
	}
}

// SetMaxConnAttempts sets the maximum number of connection attempts.
// To make it unlimited use the zero value (the default).
func (rc *ReconnectConn) SetMaxConnAttempts(maxConnAttempts int) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.maxConnAttempts = maxConnAttempts
}

func (rc *ReconnectConn) ensureConn() error {
	if rc.conn != nil {
		return nil
	}

	var err error
	for i := 0; rc.maxConnAttempts == 0 || i < rc.maxConnAttempts; i++ {
		if rc.closing.Load() {
			return net.ErrClosed
		}

		if i > 0 {
			time.Sleep(rc.retryDelay)
		}

		rc.conn, err = rc.dialer()
		if err == nil {
			_ = rc.conn.SetWriteDeadline(rc.writeDeadline)
			return nil
		}
	}

	return err
}

// Read implements net.Conn. Always fails, because ReconnectConn is write-only.
func (rc *ReconnectConn) Read(b []byte) (n int, err error) {
	return -1, errors.New("ReconnectConn is a write only connection.")
}

// Write implements net.Conn. Dials a new underlying connection if there is none.
// If the dialer function fails, retries until successfull or until the maximum
// number of attempts is reached (the dialer error is returned).
//
// If a failure occurs during write to the underlying connection, the current
// connection is closed and error is returned. A future call of Write dials
// a new connection.
func (rc *ReconnectConn) Write(b []byte) (n int, err error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if err := rc.ensureConn(); err != nil {
		return 0, err
	}

	n, err = rc.conn.Write(b)
	if err != nil {
		_ = rc.conn.Close()
		rc.conn = nil
	}

	return n, err
}

// Close implements net.Conn. Closes any active connection, cancels any currently
// running dial attempts and prevents dialing new connections.
// It is safe to call multiple times.
func (rc *ReconnectConn) Close() error {
	var err error
	rc.closeOnce.Do(func() {
		rc.closing.Store(true)
		rc.mu.Lock()
		defer rc.mu.Unlock()

		if rc.conn != nil {
			err = rc.conn.Close()
			rc.conn = nil
		}
	})
	return err
}

// LocalAddr implements net.Conn. Returns LocalAddr of the underlying conn.
// If there is no connection active, new one is dialed and its LocalAddr is returned.
// In case of failure, nil is returned.
func (rc *ReconnectConn) LocalAddr() net.Addr {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if err := rc.ensureConn(); err != nil {
		return nil
	}
	return rc.conn.LocalAddr()
}

// RemoteAddr implements net.Conn. Returns RemoteAddr of the underlying conn.
// If there is no connection active, new one is dialed and its RemoteAddr is returned.
// In case of failure, nil is returned.
func (rc *ReconnectConn) RemoteAddr() net.Addr {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if err := rc.ensureConn(); err != nil {
		return nil
	}
	return rc.conn.RemoteAddr()
}

// SetDeadline implements net.Conn. Sets write deadline of the underlying conn.
func (rc *ReconnectConn) SetDeadline(t time.Time) error {
	return rc.SetWriteDeadline(t)
}

// SetReadDeadline implements net.Conn. Always fails, because ReconnectConn is write-only.
func (rc *ReconnectConn) SetReadDeadline(t time.Time) error {
	return errors.New("ReconnectConn is a write only connection.")
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (rc *ReconnectConn) SetWriteDeadline(t time.Time) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.writeDeadline = t

	if err := rc.ensureConn(); err != nil {
		return err
	}

	return rc.conn.SetWriteDeadline(t)
}
