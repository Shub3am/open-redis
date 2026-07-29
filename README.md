# open-redis

A Redis-compatible in-memory key-value server I wrote in Go using only the standard library. It speaks RESP2, so `redis-cli`, `redis-benchmark` and ordinary Redis client libraries talk to it unchanged. It supports strings, lists, key expiry and pipelining, and its error replies match real Redis word for word.

## Supported commands

| Group | Commands | Notes |
|-------|----------|-------|
| Connection | `PING [message]`, `ECHO message` | |
| Strings | `SET key value [NX\|XX] [EX seconds\|PX milliseconds]`, `GET`, `INCR`, `DECR`, `INCRBY` | Integers must be canonical base 10, as in Redis (`05` and `+5` are rejected). Overflow is reported, not wrapped. |
| Keys | `DEL`, `EXISTS`, `EXPIRE`, `PEXPIRE`, `TTL`, `PTTL`, `KEYS pattern`, `TYPE`, `FLUSHALL [SYNC\|ASYNC]` | `KEYS` uses Redis glob rules: `*`, `?`, `[abc]`, `[^a]`, `[a-z]`, `\` escapes. |
| Lists | `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `LRANGE` (negative indexes), `LLEN` | `LPOP`/`RPOP` take no count argument. |

Arity errors, `WRONGTYPE`, syntax errors and unknown commands return the same text real Redis returns.

## Architecture

**Parser (`resp`).** The reader wraps the connection in a 64 KB `bufio.Reader` and frames one command at a time: arrays of bulk strings, plus inline commands like `PING\r\n` for people typing into `nc`. Because it reads through a buffered stream, a command split across many TCP reads and many commands packed into one read look the same to it. Array and bulk lengths are capped at the Redis defaults so a hostile header cannot force a huge allocation. Invalid input produces `-ERR Protocol error: ...` and the connection is closed, since RESP cannot be resynchronised. The writer buffers replies and only sends them on `Flush`.

**Store (`store`).** The keyspace is one `map[string]*entry` guarded by a single `sync.Mutex`. Every operation, including a read, takes the lock, because a read may delete an expired key. Each operation (for example `IncrBy` or `PushFront`) runs entirely under the lock, so check-then-write commands like `SET NX` and `INCR` are atomic without the command layer knowing about locks. Lists are `container/list` linked lists so pushes and pops at either end are O(1).

**Expiry.** TTLs live in a second map that holds only keys with an expiry. Expiry is lazy first: every lookup checks the deadline and deletes the key if it has passed, so a client can never read a stale value. A background sweep reclaims keys nobody reads again, using the same scheme as Redis: every 100 ms sample 20 keys with a TTL, delete the expired ones, and go again straight away while more than a quarter of the sample had expired. The lock is released between rounds.

**Concurrency.** The server runs one goroutine per connection, all sharing the store. Replies are flushed only when the reader has no more buffered input, so a pipeline of N commands is answered in one write. On SIGINT or SIGTERM the listener closes, every connection gets an immediate read deadline so blocked reads return, and the process exits once every connection goroutine has finished.

## Run

```sh
make run              # builds bin/open-redis and listens on 6379
make run PORT=7000
# or
go run ./cmd/open-redis -port 7000
```

```sh
$ redis-cli -p 7000 SET greeting hello EX 60
OK
$ redis-cli -p 7000 TTL greeting
(integer) 60
```

## Test

```sh
make test     # go test -race ./...
make vet
make check    # vet, race tests and a gofmt check
make bench    # Go micro-benchmarks
```

The server package has integration tests that start the server on a random port and talk raw RESP over TCP, including a pipelined batch, a command split across several writes, a protocol error and a graceful shutdown.

## Benchmarks

Measured on an Apple M5 (10 cores), macOS, Go 1.27.1, with `redis-benchmark` from Redis 8.10.2 and default settings (50 clients). The real Redis ran on the same machine with persistence off (`--save "" --appendonly no`). These are single runs on a laptop, so treat them as rough.

`redis-benchmark -p <port> -t set,get,lpush,rpush,lpop -n 100000 -q`, plus the same with `-P 16`:

| Command | open-redis | open-redis `-P 16` | Redis 8.10.2 | Redis 8.10.2 `-P 16` |
|---------|-----------:|-------------------:|-------------:|---------------------:|
| SET   | 164,204 | 1,666,667 | 208,768 | 2,272,727 |
| GET   | 170,358 | 1,851,852 | 208,333 | 2,702,703 |
| LPUSH | 183,486 | 1,694,915 | 209,644 | 2,127,660 |
| RPUSH | 164,204 | 1,694,915 | 202,020 | 2,380,953 |
| LPOP  | 171,527 | 2,439,024 | 205,339 | 2,083,333 |

Requests per second. Without pipelining open-redis runs at roughly 80 to 90 percent of Redis. `redis-benchmark` prints `WARNING: Could not fetch server CONFIG` against open-redis because `CONFIG` is not implemented; the benchmark still runs.

Go micro-benchmarks (`go test -run '^$' -bench . -benchmem ./...`):

```
BenchmarkReadCommand-10        	 7637259	       154.8 ns/op	 484.64 MB/s	     187 B/op	      11 allocs/op
BenchmarkWriteBulkString-10    	100000000	        11.54 ns/op	       0 B/op	       0 allocs/op
BenchmarkPushFront-10      	26853079	        50.35 ns/op	      64 B/op	       2 allocs/op
BenchmarkSet-10            	37457566	        29.02 ns/op	      48 B/op	       1 allocs/op
BenchmarkGet-10            	87630949	        13.32 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetParallel-10    	11397450	       101.6 ns/op	       0 B/op	       0 allocs/op
```

`BenchmarkGetParallel` is slower per operation than `BenchmarkGet` because every goroutine contends on the one keyspace mutex. That is the first thing I would change for more throughput on many cores, by sharding the keyspace by key hash.

## Limitations

No persistence, replication, pub/sub, transactions, blocking list commands or RESP3. `KEYS` scans the whole keyspace under the lock, as it does in Redis, so avoid it on large datasets.
