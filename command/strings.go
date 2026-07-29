// This file holds the connection commands and the commands that read or write
// string values. It must not touch list values.
package command

import (
	"math"
	"strings"
	"time"

	"github.com/Shub3am/open-redis/resp"
	"github.com/Shub3am/open-redis/store"
)

func ping(_ *store.Store, args []string, reply *resp.Writer) {
	switch len(args) {
	case 0:
		reply.WriteSimpleString("PONG")
	case 1:
		reply.WriteBulkString(args[0])
	default:
		reply.WriteError("ERR wrong number of arguments for 'ping' command")
	}
}

func echo(_ *store.Store, args []string, reply *resp.Writer) {
	reply.WriteBulkString(args[0])
}

func get(keyspace *store.Store, args []string, reply *resp.Writer) {
	value, found, err := keyspace.Get(args[0])
	writeOptionalValue(reply, value, found, err)
}

// set parses SET key value [NX|XX] [EX seconds|PX milliseconds]. Flags may
// come in any order, but NX with XX or EX with PX is a syntax error.
func set(keyspace *store.Store, args []string, reply *resp.Writer) {
	key, value := args[0], args[1]
	var options store.SetOptions
	hasTTL := false
	for i := 2; i < len(args); i++ {
		switch flag := strings.ToUpper(args[i]); flag {
		case "NX":
			if options.OnlyIfExists {
				reply.WriteError(errSyntax)
				return
			}
			options.OnlyIfMissing = true
		case "XX":
			if options.OnlyIfMissing {
				reply.WriteError(errSyntax)
				return
			}
			options.OnlyIfExists = true
		case "EX", "PX":
			if hasTTL || i+1 == len(args) {
				reply.WriteError(errSyntax)
				return
			}
			amount, ok := store.ParseInteger(args[i+1])
			if !ok {
				reply.WriteError(errNotInteger)
				return
			}
			unit := time.Second
			if flag == "PX" {
				unit = time.Millisecond
			}
			if amount <= 0 || amount > math.MaxInt64/int64(unit) {
				reply.WriteError("ERR invalid expire time in 'set' command")
				return
			}
			options.TTL = time.Duration(amount) * unit
			hasTTL = true
			i++
		default:
			reply.WriteError(errSyntax)
			return
		}
	}

	if !keyspace.Set(key, value, options) {
		reply.WriteNullBulkString()
		return
	}
	reply.WriteSimpleString("OK")
}

func incr(keyspace *store.Store, args []string, reply *resp.Writer) {
	writeIncrement(keyspace, args[0], 1, reply)
}

func decr(keyspace *store.Store, args []string, reply *resp.Writer) {
	writeIncrement(keyspace, args[0], -1, reply)
}

func incrBy(keyspace *store.Store, args []string, reply *resp.Writer) {
	delta, ok := store.ParseInteger(args[1])
	if !ok {
		reply.WriteError(errNotInteger)
		return
	}
	writeIncrement(keyspace, args[0], delta, reply)
}

func writeIncrement(keyspace *store.Store, key string, delta int64, reply *resp.Writer) {
	next, err := keyspace.IncrBy(key, delta)
	if err != nil {
		reply.WriteError(err.Error())
		return
	}
	reply.WriteInteger(next)
}
