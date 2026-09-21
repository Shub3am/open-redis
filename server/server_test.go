package server

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Shub3am/open-redis/store"
)

// startServer runs Serve on a random local port and stops it when the test
// ends. It fails the test if Serve does not return cleanly.
func startServer(t *testing.T) (address string, stop func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- Serve(ctx, listener, store.New()) }()

	stopped := false
	stop = func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-served:
			if err != nil {
				t.Errorf("Serve returned %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Serve did not return after cancellation")
		}
	}
	t.Cleanup(stop)
	return listener.Addr().String(), stop
}

func dial(t *testing.T, address string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	return conn, bufio.NewReader(conn)
}

func send(t *testing.T, conn net.Conn, raw string) {
	t.Helper()
	if _, err := io.WriteString(conn, raw); err != nil {
		t.Fatal(err)
	}
}

func expectReply(t *testing.T, replies *bufio.Reader, want string) {
	t.Helper()
	got := make([]byte, len(want))
	if _, err := io.ReadFull(replies, got); err != nil {
		t.Fatalf("reading reply %q: %v (got %q)", want, err, got)
	}
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPipelinedCommandsInOneWrite(t *testing.T) {
	address, _ := startServer(t)
	conn, replies := dial(t, address)

	send(t, conn, "*1\r\n$4\r\nPING\r\n"+
		"*3\r\n$3\r\nSET\r\n$5\r\ngreet\r\n$5\r\nhello\r\n"+
		"*2\r\n$3\r\nGET\r\n$5\r\ngreet\r\n"+
		"*3\r\n$5\r\nRPUSH\r\n$4\r\nlist\r\n$1\r\na\r\n"+
		"*2\r\n$3\r\nGET\r\n$4\r\nlist\r\n"+
		"INCR counter\r\n")

	expectReply(t, replies, "+PONG\r\n"+
		"+OK\r\n"+
		"$5\r\nhello\r\n"+
		":1\r\n"+
		"-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"+
		":1\r\n")
}

func TestCommandSplitAcrossWrites(t *testing.T) {
	address, _ := startServer(t)
	conn, replies := dial(t, address)

	command := "*2\r\n$4\r\nECHO\r\n$11\r\nhello world\r\n"
	for _, chunk := range []string{command[:3], command[3:17], command[17:]} {
		send(t, conn, chunk)
		time.Sleep(10 * time.Millisecond)
	}
	expectReply(t, replies, "$11\r\nhello world\r\n")
}

func TestClientsShareOneKeyspace(t *testing.T) {
	address, _ := startServer(t)
	writer, writerReplies := dial(t, address)
	reader, readerReplies := dial(t, address)

	send(t, writer, "SET shared value\r\n")
	expectReply(t, writerReplies, "+OK\r\n")
	send(t, reader, "GET shared\r\n")
	expectReply(t, readerReplies, "$5\r\nvalue\r\n")
}

func TestProtocolErrorClosesConnection(t *testing.T) {
	address, _ := startServer(t)
	conn, replies := dial(t, address)

	send(t, conn, "*1\r\n:1\r\n")
	expectReply(t, replies, "-ERR Protocol error: expected '$', got ':'\r\n")
	if _, err := replies.ReadByte(); err != io.EOF {
		t.Fatalf("want connection closed, got %v", err)
	}
}

func TestShutdownClosesIdleConnections(t *testing.T) {
	address, stop := startServer(t)
	conn, replies := dial(t, address)
	send(t, conn, "PING\r\n")
	expectReply(t, replies, "+PONG\r\n")

	stop()

	if _, err := replies.ReadByte(); err != io.EOF {
		t.Fatalf("want connection closed on shutdown, got %v", err)
	}
	if _, err := net.Dial("tcp", address); err == nil {
		t.Fatal("listener still accepting after shutdown")
	}
}

func TestLargeValueRoundTrip(t *testing.T) {
	address, _ := startServer(t)
	conn, replies := dial(t, address)

	value := strings.Repeat("x", 200_000)
	send(t, conn, "*3\r\n$3\r\nSET\r\n$3\r\nbig\r\n$200000\r\n"+value+"\r\n")
	expectReply(t, replies, "+OK\r\n")
	send(t, conn, "GET big\r\n")
	expectReply(t, replies, "$200000\r\n"+value+"\r\n")
}
