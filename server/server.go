// Package server accepts TCP clients and runs their commands against a shared
// keyspace, one goroutine per connection. It owns connection lifecycle and
// shutdown. It must not interpret commands or bytes itself; that is the job of
// command and resp.
package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/Shub3am/open-redis/command"
	"github.com/Shub3am/open-redis/resp"
	"github.com/Shub3am/open-redis/store"
)

// Serve accepts connections on listener until ctx is cancelled or Accept
// fails. On return the listener is closed and every connection goroutine has
// finished. Cancellation is a clean stop and returns nil.
func Serve(ctx context.Context, listener net.Listener, keyspace *store.Store) error {
	var connections sync.WaitGroup
	defer connections.Wait()
	// Cancelling this context on any return is what tells open connections to
	// stop, so the deferred Wait above cannot block forever.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stopClosingListener := context.AfterFunc(ctx, func() { listener.Close() })
	defer stopClosingListener()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		connections.Add(1)
		go func() {
			defer connections.Done()
			serveConnection(ctx, conn, keyspace)
		}()
	}
}

// serveConnection runs one client's commands in order until it disconnects,
// sends invalid RESP, or ctx is cancelled.
func serveConnection(ctx context.Context, conn net.Conn, keyspace *store.Store) {
	defer conn.Close()
	// An expired read deadline wakes a blocked read without cutting off a
	// reply that is still being written.
	stopInterrupt := context.AfterFunc(ctx, func() { conn.SetReadDeadline(time.Now()) })
	defer stopInterrupt()

	reader := resp.NewReader(conn)
	reply := resp.NewWriter(conn)
	for {
		args, err := reader.ReadCommand()
		if err != nil {
			var protocolErr *resp.ProtocolError
			if errors.As(err, &protocolErr) {
				reply.WriteError("ERR " + protocolErr.Error())
				reply.Flush()
			}
			return
		}
		command.Execute(keyspace, args, reply)
		// Flushing only once the pipeline is drained sends a batch of replies
		// in one write instead of one write per command.
		if reader.Buffered() == 0 {
			if err := reply.Flush(); err != nil {
				return
			}
		}
	}
}
