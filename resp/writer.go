// This file encodes replies in RESP2. It buffers everything until Flush so a
// pipeline of replies leaves in as few writes as possible. It must not decide
// which reply a command gets.
package resp

import (
	"bufio"
	"io"
	"strconv"
)

// Writer encodes replies. Write errors are sticky inside bufio.Writer and
// surface on Flush, which is why the individual methods return nothing.
type Writer struct {
	buffered *bufio.Writer
	scratch  []byte
}

func NewWriter(destination io.Writer) *Writer {
	return &Writer{buffered: bufio.NewWriter(destination)}
}

func (w *Writer) WriteSimpleString(text string) {
	w.writeLine('+', text)
}

// WriteError writes an error reply. The message should start with an error
// code such as "ERR" or "WRONGTYPE", as clients branch on it.
func (w *Writer) WriteError(message string) {
	w.writeLine('-', message)
}

func (w *Writer) WriteInteger(number int64) {
	w.writeLength(':', number)
}

func (w *Writer) WriteBulkString(text string) {
	w.writeLength('$', int64(len(text)))
	w.buffered.WriteString(text)
	w.buffered.WriteString("\r\n")
}

// WriteNullBulkString writes the reply for a missing value, "$-1".
func (w *Writer) WriteNullBulkString() {
	w.buffered.WriteString("$-1\r\n")
}

// WriteArray writes an array whose elements are all bulk strings, which is the
// only array shape the supported commands return.
func (w *Writer) WriteArray(items []string) {
	w.writeLength('*', int64(len(items)))
	for _, item := range items {
		w.WriteBulkString(item)
	}
}

func (w *Writer) Flush() error {
	return w.buffered.Flush()
}

func (w *Writer) writeLine(prefix byte, text string) {
	w.buffered.WriteByte(prefix)
	w.buffered.WriteString(text)
	w.buffered.WriteString("\r\n")
}

func (w *Writer) writeLength(prefix byte, number int64) {
	w.scratch = append(w.scratch[:0], prefix)
	w.scratch = strconv.AppendInt(w.scratch, number, 10)
	w.scratch = append(w.scratch, '\r', '\n')
	w.buffered.Write(w.scratch)
}
