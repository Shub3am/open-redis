package resp

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

func readAllCommands(t *testing.T, source io.Reader) ([][]string, error) {
	t.Helper()
	reader := NewReader(source)
	var commands [][]string
	for {
		args, err := reader.ReadCommand()
		if err != nil {
			return commands, err
		}
		commands = append(commands, args)
	}
}

func TestReadCommandFramesArraysAndInline(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  [][]string
	}{
		{"array", "*2\r\n$4\r\nECHO\r\n$5\r\nhello\r\n", [][]string{{"ECHO", "hello"}}},
		{"empty bulk string", "*2\r\n$4\r\nECHO\r\n$0\r\n\r\n", [][]string{{"ECHO", ""}}},
		{"binary payload with CRLF inside", "*2\r\n$4\r\nECHO\r\n$4\r\na\r\nb\r\n", [][]string{{"ECHO", "a\r\nb"}}},
		{"inline", "SET  key value\r\n", [][]string{{"SET", "key", "value"}}},
		{"inline with bare newline", "PING\n", [][]string{{"PING"}}},
		{"empty array and blank line are skipped", "*0\r\n\r\n*1\r\n$4\r\nPING\r\n", [][]string{{"PING"}}},
		{
			"pipelined",
			"*1\r\n$4\r\nPING\r\n*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\nGET k\r\n",
			[][]string{{"PING"}, {"SET", "k", "v"}, {"GET", "k"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := readAllCommands(t, strings.NewReader(test.input))
			if !errors.Is(err, io.EOF) {
				t.Fatalf("want clean io.EOF at end, got %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestReadCommandHandlesOneByteReads(t *testing.T) {
	input := "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n*1\r\n$4\r\nPING\r\n"
	got, err := readAllCommands(t, iotest.OneByteReader(strings.NewReader(input)))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("want io.EOF, got %v", err)
	}
	want := [][]string{{"SET", "key", "value"}, {"PING"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReadCommandReportsTruncatedInput(t *testing.T) {
	for _, input := range []string{"*2\r\n$4\r\nECHO\r\n", "*1\r\n$4\r\nPI", "*1\r\n$4"} {
		_, err := NewReader(strings.NewReader(input)).ReadCommand()
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("input %q: want io.ErrUnexpectedEOF, got %v", input, err)
		}
	}
}

func TestReadCommandRejectsMalformedInput(t *testing.T) {
	tests := map[string]string{
		"*x\r\n":                   "Protocol error: invalid multibulk length",
		"*2000000\r\n":             "Protocol error: invalid multibulk length",
		"*1\r\n:4\r\n":             "Protocol error: expected '$', got ':'",
		"*1\r\n$-1\r\n":            "Protocol error: invalid bulk length",
		"*1\r\n$999999999999\r\n":  "Protocol error: invalid bulk length",
		"*1\r\n$4\r\nPINGxx":       "Protocol error: bulk string not terminated by CRLF",
		strings.Repeat("a", 70000): "Protocol error: too big request line",
	}
	for input, want := range tests {
		_, err := NewReader(strings.NewReader(input)).ReadCommand()
		var protocolErr *ProtocolError
		if !errors.As(err, &protocolErr) || err.Error() != want {
			t.Errorf("input %.20q: got %v, want %q", input, err, want)
		}
	}
}

func BenchmarkReadCommand(b *testing.B) {
	command := "*3\r\n$3\r\nSET\r\n$16\r\nkey:000000000001\r\n$32\r\nvalue-value-value-value-value-va\r\n"
	stream := strings.Repeat(command, 1000)
	b.SetBytes(int64(len(command)))
	b.ReportAllocs()
	source := strings.NewReader(stream)
	reader := NewReader(source)
	for i := 0; b.Loop(); i++ {
		if i%1000 == 0 {
			source.Reset(stream)
		}
		if _, err := reader.ReadCommand(); err != nil {
			b.Fatal(err)
		}
	}
}
