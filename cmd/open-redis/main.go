// Command open-redis runs the server. It only wires flags, signals and
// packages together; it must not contain server behaviour of its own.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Shub3am/open-redis/server"
	"github.com/Shub3am/open-redis/store"
)

const activeExpiryInterval = 100 * time.Millisecond

func main() {
	port := flag.Int("port", 6379, "TCP port to listen on")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("listen on port %d: %v", *port, err)
	}

	keyspace := store.New()
	go keyspace.RunActiveExpiry(ctx, activeExpiryInterval)

	log.Printf("open-redis listening on %s", listener.Addr())
	if err := server.Serve(ctx, listener, keyspace); err != nil {
		log.Fatalf("serve: %v", err)
	}
	log.Print("open-redis shut down")
}
