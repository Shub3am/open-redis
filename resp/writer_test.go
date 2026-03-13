package resp

import (
	"bytes"
	"io"
	"testing"
)

func TestWriterEncodesEveryReplyType(t *testing.T) {
	var output bytes.Buffer
	writer := NewWriter(&output)

	writer.WriteSimpleString("OK")
	writer.WriteError("ERR boom")
	writer.WriteInteger(-42)
	writer.WriteBulkString("hello")
	writer.WriteBulkString("")
	writer.WriteNullBulkString()
	writer.WriteArray([]string{"a", "bc"})
	writer.WriteArray(nil)

	if output.Len() != 0 {
		t.Fatalf("writer sent %q before Flush", output.String())
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	want := "+OK\r\n" +
		"-ERR boom\r\n" +
		":-42\r\n" +
		"$5\r\nhello\r\n" +
		"$0\r\n\r\n" +
		"$-1\r\n" +
		"*2\r\n$1\r\na\r\n$2\r\nbc\r\n" +
		"*0\r\n"
	if output.String() != want {
		t.Fatalf("got %q, want %q", output.String(), want)
	}
}

func BenchmarkWriteBulkString(b *testing.B) {
	writer := NewWriter(io.Discard)
	b.ReportAllocs()
	for b.Loop() {
		writer.WriteBulkString("value-value-value-value-value-va")
	}
	if err := writer.Flush(); err != nil {
		b.Fatal(err)
	}
}
