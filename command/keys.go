// This file holds the commands that act on keys regardless of their value
// type: existence, deletion, expiry and inspection. It must not read or write
// the values themselves.
package command

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Shub3am/open-redis/resp"
	"github.com/Shub3am/open-redis/store"
)

func del(keyspace *store.Store, args []string, reply *resp.Writer) {
	reply.WriteInteger(int64(keyspace.Delete(args...)))
}

func exists(keyspace *store.Store, args []string, reply *resp.Writer) {
	reply.WriteInteger(int64(keyspace.Exists(args...)))
}

func expire(keyspace *store.Store, args []string, reply *resp.Writer) {
	writeExpire(keyspace, args, time.Second, "expire", reply)
}

func pexpire(keyspace *store.Store, args []string, reply *resp.Writer) {
	writeExpire(keyspace, args, time.Millisecond, "pexpire", reply)
}

func writeExpire(keyspace *store.Store, args []string, unit time.Duration, name string, reply *resp.Writer) {
	amount, ok := store.ParseInteger(args[1])
	if !ok {
		reply.WriteError(errNotInteger)
		return
	}
	if amount > math.MaxInt64/int64(unit) || amount < math.MinInt64/int64(unit) {
		reply.WriteError(fmt.Sprintf("ERR invalid expire time in '%s' command", name))
		return
	}
	if keyspace.Expire(args[0], time.Duration(amount)*unit) {
		reply.WriteInteger(1)
		return
	}
	reply.WriteInteger(0)
}

func ttl(keyspace *store.Store, args []string, reply *resp.Writer) {
	writeTTL(keyspace, args[0], time.Second, reply)
}

func pttl(keyspace *store.Store, args []string, reply *resp.Writer) {
	writeTTL(keyspace, args[0], time.Millisecond, reply)
}

// writeTTL replies -2 for a missing key and -1 for a key without expiry, as
// Redis does. Seconds are rounded to the nearest whole second, also as Redis
// does, so a key set with EX 10 reports 10 rather than 9.
func writeTTL(keyspace *store.Store, key string, unit time.Duration, reply *resp.Writer) {
	remaining, hasExpiry, exists := keyspace.TTL(key)
	switch {
	case !exists:
		reply.WriteInteger(-2)
	case !hasExpiry:
		reply.WriteInteger(-1)
	default:
		reply.WriteInteger(int64((remaining + unit/2) / unit))
	}
}

func keys(keyspace *store.Store, args []string, reply *resp.Writer) {
	reply.WriteArray(keyspace.Keys(args[0]))
}

func keyType(keyspace *store.Store, args []string, reply *resp.Writer) {
	reply.WriteSimpleString(string(keyspace.Type(args[0])))
}

// flushAll accepts the SYNC and ASYNC modifiers real clients send. Both flush
// synchronously here because the keyspace has no background freeing.
func flushAll(keyspace *store.Store, args []string, reply *resp.Writer) {
	if len(args) > 1 || (len(args) == 1 && !strings.EqualFold(args[0], "SYNC") && !strings.EqualFold(args[0], "ASYNC")) {
		reply.WriteError(errSyntax)
		return
	}
	keyspace.FlushAll()
	reply.WriteSimpleString("OK")
}
