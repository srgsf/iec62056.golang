package iec62056

import (
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"
)

type mockConn struct {
	io.ReadWriteCloser
	closed                      bool
	readDeadLine, writeDeadLine time.Time
	rBuf, wBuf                  bytes.Buffer
}

func (b *mockConn) LocalAddr() net.Addr         { return nil }
func (b *mockConn) RemoteAddr() net.Addr        { return nil }
func (b *mockConn) SetDeadline(time.Time) error { return nil }

func (b *mockConn) Close() error {
	b.closed = true
	return nil
}

func (b *mockConn) SetReadDeadline(t time.Time) error {
	b.readDeadLine = t
	return nil
}

func (b *mockConn) SetWriteDeadline(t time.Time) error {
	b.writeDeadLine = t
	return nil
}

func (b *mockConn) Read(p []byte) (int, error) {
	return b.rBuf.Read(p)
}

func (b *mockConn) Write(p []byte) (int, error) {
	return b.wBuf.Write(p)
}

func TestClose(t *testing.T) {
	var c mockConn
	conn := newConn(&c, nil, false, 3*time.Second)
	c.closed = false
	_ = conn.Close()
	if !c.closed {
		t.Error("Close method doesn't closes wrapped connection")
	}
}

func TestSetReadTimeout(t *testing.T) {
	var c mockConn
	rdl := 3 * time.Minute
	round := time.Minute
	conn := newConn(&c, nil, false, rdl)
	c.readDeadLine = time.Time{}
	tt := time.Now().Add(rdl).Round(round)
	_ = conn.prepareRead()
	dd := c.readDeadLine.Round(round)
	if dd != tt {
		t.Error("frame read deadline is not properly set")
	}
}

func TestSetWriteTimeout(t *testing.T) {
	var c mockConn
	wdl := 3 * time.Minute
	round := time.Minute
	conn := newConn(&c, nil, false, wdl)
	c.writeDeadLine = time.Time{}
	tt := time.Now().Add(wdl).Round(round)
	_ = conn.prepareWrite()
	dd := c.writeDeadLine.Round(round)
	if dd != tt {
		t.Error("frame write deadline is not properly set")
	}
}

func TestNoLoggerRead(t *testing.T) {
	var c mockConn
	conn := newConn(&c, nil,false, 3*time.Second)
	_ = conn.prepareRead()
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	c.rBuf.Write(payload)
	res := make([]byte, len(payload))
	_ = conn.prepareRead()
	_, _ = conn.read(res)
	res = conn.r.buf.Bytes()
	if len(res) != 0 {
		t.Error("frame is saved even without logger configured.")
	}
}

func TestNoLoggerWrite(t *testing.T) {
	var c mockConn
	conn := newConn(&c, nil,false, 3*time.Second)
	_ = conn.prepareWrite()
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	_, _ = conn.write(payload)
	res := conn.w.buf.Bytes()
	if len(res) != 0 {
		t.Error("frame is saved even without logger configured.")
	}
}

func TestLoggerRead(t *testing.T) {
	var c mockConn
	conn := newConn(&c, log.New(os.Stderr, "", log.Ldate),false, 3*time.Second)
	_ = conn.prepareRead()
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	c.rBuf.Write(payload)
	res := make([]byte, len(payload))
	_ = conn.prepareRead()
	_, _ = conn.read(res)
	res = conn.r.buf.Bytes()
	if len(res) != len(payload) {
		t.Error("payload and log lengths aren't match")
	}
	for i := range payload {
		if payload[i] != res[i] {
			t.Error("frame data and log aren't match")
		}
	}
}

func TestLoggerWrite(t *testing.T) {
	var c mockConn
	conn := newConn(&c, log.New(os.Stderr, "", log.Ldate),false, 3*time.Second)
	_ = conn.prepareWrite()
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	_, _ = conn.write(payload)
	res := conn.w.buf.Bytes()
	if len(payload) != len(res) {
		t.Error("payload and log lengths aren't match")
	}
	for i := range payload {
		if payload[i] != res[i] {
			t.Error("buffered message incorrectly logged")
		}
	}
}

func TestLoggerReadReset(t *testing.T) {
	var c mockConn
	conn := newConn(&c, log.New(os.Stderr, "", log.Ldate),false, 3*time.Second)
	_ = conn.prepareRead()
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	c.rBuf.Write(payload)
	res := make([]byte, len(payload))
	_ = conn.prepareRead()
	_, _ = conn.read(res)
	res = conn.r.buf.Bytes()
	if len(res) != len(payload) {
		t.Error("payload and log lengths aren't match")
	}
	_ = conn.prepareRead()
	res = conn.r.buf.Bytes()
	if len(res) != 0 {
		t.Error("frame isn't reset")
	}
}

func TestLoggerWriteReset(t *testing.T) {
	var c mockConn
	conn := newConn(&c, log.New(os.Stderr, "", log.Ldate),false, 3*time.Second)
	_ = conn.prepareWrite()
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	_, _ = conn.write(payload)
	res := conn.w.buf.Bytes()
	if len(payload) != len(res) {
		t.Error("payload and log lengths aren't match")
	}
	_ = conn.prepareWrite()
	res = conn.w.buf.Bytes()
	if len(res) != 0 {
		t.Error("frame isn't reset")
	}
}
