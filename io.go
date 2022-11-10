package iec62056

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"math/bits"
	"net"
	"strconv"
	"strings"
	"time"
)

// default i/o frame operations timeout
const timeout = time.Second * 5

// Conn is a network connection handle
type Conn struct {
	// wrapped connection
	rwc net.Conn
	// operations wrapper
	io io.ReadWriter
	// i/o operations timeout
	to time.Duration
	// buffered reader handler.
	r reader
	//buffered writer handler
	w writer
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.rwc.Close()
}

// prepareRead configures frame reading operation. Call it once before frame sequential reads.
func (c *Conn) prepareRead() error {
	c.r.reset(c.io)
	if err := c.rwc.SetReadDeadline(time.Now().Add(c.to)); err != nil {
		return err
	}
	return nil
}

// prepareWrite configures frame writing operation. Call it once before frame sequential writes.
func (c *Conn) prepareWrite() error {
	c.w.reset(c.io)
	if err := c.rwc.SetWriteDeadline(time.Now().Add(c.to)); err != nil {
		return err
	}
	return nil
}

// logs received frame
func (c *Conn) logResponse() {
	c.r.log("response")
}

// logs written frame
func (c *Conn) logRequest() {
	c.w.log("request")
}

// A Dialer contains options for connecting to a network.
type Dialer struct {
	// Tcp socket connection timeout.
	ConnectionTimeOut time.Duration
	// I/O frame operations timeout.
	RWTimeOut time.Duration
	// Logger for received and sent frames.
	ProtocolLogger *log.Logger
	// If true then even partiy translation is applied on reads and writes.
	SwParity bool
}

// DialTCP connects to the tcp socket on the named network.
// The socket has the form "host:port".
func DialTCP(socket string) (*Conn, error) {
	var d Dialer
	return d.DialTCP(socket)
}

// DialTCP connects to the tcp socket on the named network.
// The socket has the form "host:port".
func (d *Dialer) DialTCP(socket string) (*Conn, error) {
	conn, err := net.DialTimeout("tcp", socket, d.ConnectionTimeOut)
	if err != nil {
		return nil, err
	}

	var to = d.RWTimeOut
	if to == 0 {
		to = timeout
	}
	return newConn(conn, d.ProtocolLogger, d.SwParity, to), nil
}

// creates connection.
func newConn(conn net.Conn, log *log.Logger, swParity bool, to time.Duration) *Conn {
	var l = &logger{
		l: log,
	}
	var io io.ReadWriter = conn
	if swParity {
		io = &parityWrapper{io: conn}
	}

	return &Conn{
		conn,
		io,
		to,
		reader{
			l,
			bufio.NewReader(io),
		},
		writer{
			l,
			bufio.NewWriter(io),
		},
	}
}

type parityWrapper struct {
	io io.ReadWriter
}

func (w *parityWrapper) Read(p []byte) (int, error) {
	n, err := w.io.Read(p)
	if err != nil {
		return n, err
	}
	for i := 0; i < n; i++ {
		p[i] &= 0x7f
	}
	return n, err
}

func (w *parityWrapper) Write(p []byte) (int, error) {
	p2 := make([]byte, len(p))
	copy(p2, p)
	for i, b := range p {
		if bits.OnesCount8(b)&0x1 == 1 {
			p2[i] |= 0x80
		}
	}
	p = p2
	return w.io.Write(p)
}

// Frame logger
type logger struct {
	// buffer for partial reads writes.
	buf bytes.Buffer
	// logger
	l *log.Logger
}

// log logs read or written frame. Contents are reset on prepareRead or prepareWrite methods call.
func (l *logger) log(prefix string) {
	if l.l != nil {
		l.l.Println(formatMsg(prefix, l.buf.Bytes()))
	}
	l.buf.Reset()
}

// Buffered reader that logs read bytes.
type reader struct {
	*logger
	*bufio.Reader
}

// reset discards any buffered data. Also resets collected frame's log message.
func (b *reader) reset(r io.Reader) {
	b.Reader.Reset(r)
	b.logger.buf.Reset()
}

// io.Reader interface implementation.
// Read reads data into p and appends it to frame's log message.
func (b *reader) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == nil && b.l != nil {
		_, err = b.logger.buf.Write(p)
	}
	return n, err
}

// bufio.Reader interface implementation.
// ReadByte reads a byte and appends it to frame's log message.
func (b *reader) ReadByte() (byte, error) {
	n, err := b.Reader.ReadByte()
	if err == nil && b.l != nil {
		_ = b.logger.buf.WriteByte(n)
	}
	return n, err
}

// bufio.Reader interface implementation.
//
//	ReadBytes reads until the first occurrence of delim in the input and appends returned data to log buffer.
func (b *reader) ReadBytes(delim byte) ([]byte, error) {
	data, err := b.Reader.ReadBytes(delim)
	if err == nil && b.l != nil {
		_, err = b.logger.buf.Write(data)
	}
	return data, err
}

// Buffered writer that logs written bytes
type writer struct {
	*logger
	*bufio.Writer
}

// reset discards any buffered data. Also resets collected frame's log message.
func (b *writer) reset(w io.Writer) {
	b.Writer.Reset(w)
	b.logger.buf.Reset()
}

// io.Writer implementation.
// Write writes data from p into the socket.
func (b *writer) Write(p []byte) (int, error) {
	nn, err := b.Writer.Write(p)
	if err == nil && b.l != nil {
		_, err = b.logger.buf.Write(p)
	}
	return nn, err
}

// bufio.Writer implementation.
// WriteByte writes a byte into the socket.
func (b *writer) WriteByte(p byte) error {
	err := b.Writer.WriteByte(p)
	if err == nil && b.l != nil {
		_ = b.logger.buf.WriteByte(p)
	}
	return err
}

// formats frame log as two areas. On the left side frame bytes as hex bytes, on the right is a string representation.
func formatMsg(prefix string, data []byte) string {
	var b1 strings.Builder

	b1.WriteString(prefix)
	b1.WriteRune('\n')
	for i := 0; i < len(data); i += 16 {
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		for _, b := range data[i:end] {
			_, _ = fmt.Fprintf(&b1, "%02X ", b)
		}
		b1.WriteString(strings.Repeat(" ", 58-(3*(end-i))))
		b1.WriteString(strings.Map(mapNotPrintable, string(data[i:end])))
		b1.WriteRune('\n')
	}

	return b1.String()
}

// replaces non-printable runes with dots '.'
func mapNotPrintable(r rune) rune {
	if strconv.IsPrint(r) {
		return r
	}
	return '.'
}
