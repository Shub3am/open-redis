// Package resp speaks the RESP2 wire protocol that Redis clients use.
//
// This file turns a byte stream into commands. It must not know what any
// command means; it only frames them.
package resp

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// Limits match the defaults of a real Redis server so a hostile header cannot
// make us allocate unbounded memory.
const (
	maxArrayLength   = 1024 * 1024
	maxBulkLength    = 512 * 1024 * 1024
	readerBufferSize = 64 * 1024
)

// ProtocolError means the client sent bytes that are not valid RESP. The
// connection cannot be resynchronised after one, so the caller should reply
// and close.
type ProtocolError struct {
	reason string
}

func (e *ProtocolError) Error() string {
	return "Protocol error: " + e.reason
}

// Reader frames commands from a client connection. It handles commands that
// arrive split across many reads as well as many commands arriving in one read.
type Reader struct {
	buffered *bufio.Reader
}

func NewReader(source io.Reader) *Reader {
	return &Reader{buffered: bufio.NewReaderSize(source, readerBufferSize)}
}

// Buffered reports how many unread bytes are already in memory. Zero means the
// client's pipeline is drained and it is a good moment to flush replies.
func (r *Reader) Buffered() int {
	return r.buffered.Buffered()
}

// ReadCommand returns the next command as its arguments, name first. It
// accepts both arrays of bulk strings and inline commands such as "PING\r\n".
// Empty commands are skipped, as Redis does. It returns io.EOF only when the
// stream ends cleanly between commands.
func (r *Reader) ReadCommand() ([]string, error) {
	for {
		line, err := r.readLine()
		if err != nil {
			return nil, err
		}
		if len(line) == 0 {
			continue
		}
		if line[0] != '*' {
			args := strings.Fields(line)
			if len(args) == 0 {
				continue
			}
			return args, nil
		}

		count, err := strconv.Atoi(line[1:])
		if err != nil || count > maxArrayLength {
			return nil, &ProtocolError{reason: "invalid multibulk length"}
		}
		if count <= 0 {
			continue
		}
		args, err := r.readBulkStrings(count)
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return args, err
	}
}

func (r *Reader) readBulkStrings(count int) ([]string, error) {
	args := make([]string, 0, min(count, 1024))
	for range count {
		arg, err := r.readBulkString()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	return args, nil
}

func (r *Reader) readBulkString() (string, error) {
	header, err := r.readLine()
	if err != nil {
		return "", err
	}
	if len(header) == 0 || header[0] != '$' {
		got := ""
		if len(header) > 0 {
			got = header[:1]
		}
		return "", &ProtocolError{reason: "expected '$', got '" + got + "'"}
	}
	length, err := strconv.Atoi(header[1:])
	if err != nil || length < 0 || length > maxBulkLength {
		return "", &ProtocolError{reason: "invalid bulk length"}
	}

	payload := make([]byte, length+2)
	if _, err := io.ReadFull(r.buffered, payload); err != nil {
		return "", err
	}
	if payload[length] != '\r' || payload[length+1] != '\n' {
		return "", &ProtocolError{reason: "bulk string not terminated by CRLF"}
	}
	return string(payload[:length]), nil
}

// readLine returns one line without its terminator. Redis tolerates a bare
// "\n" from hand-typed inline commands, so the "\r" is optional.
func (r *Reader) readLine() (string, error) {
	line, err := r.buffered.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return "", &ProtocolError{reason: "too big request line"}
	}
	if err != nil {
		if errors.Is(err, io.EOF) && len(line) > 0 {
			return "", io.ErrUnexpectedEOF
		}
		return "", err
	}
	line = line[:len(line)-1]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return string(line), nil
}
