// Package command turns a parsed command into a keyspace operation and a
// reply. It owns argument validation and the exact reply shapes clients
// expect. It must not read from the network or hold keyspace locks itself;
// framing lives in resp and atomicity lives in store.
package command

import (
	"fmt"
	"strings"

	"github.com/Shub3am/open-redis/resp"
	"github.com/Shub3am/open-redis/store"
)

const (
	errSyntax     = "ERR syntax error"
	errNotInteger = "ERR value is not an integer or out of range"
)

// handler receives the arguments after the command name, already checked
// against the command's arity.
type handler func(keyspace *store.Store, args []string, reply *resp.Writer)

// spec pairs a handler with its arity, counted the way Redis counts it: the
// name included, positive for an exact count, negative for a minimum.
type spec struct {
	arity int
	run   handler
}

var commands = map[string]spec{
	"ping":   {-1, ping},
	"echo":   {2, echo},
	"set":    {-3, set},
	"get":    {2, get},
	"incr":   {2, incr},
	"decr":   {2, decr},
	"incrby": {3, incrBy},

	"del":      {-2, del},
	"exists":   {-2, exists},
	"expire":   {3, expire},
	"pexpire":  {3, pexpire},
	"ttl":      {2, ttl},
	"pttl":     {2, pttl},
	"keys":     {2, keys},
	"type":     {2, keyType},
	"flushall": {-1, flushAll},

	"lpush":  {-3, lpush},
	"rpush":  {-3, rpush},
	"lpop":   {2, lpop},
	"rpop":   {2, rpop},
	"lrange": {4, lrange},
	"llen":   {2, llen},
}

// Execute runs one command and writes exactly one reply. args must hold at
// least the command name.
func Execute(keyspace *store.Store, args []string, reply *resp.Writer) {
	name := strings.ToLower(args[0])
	command, known := commands[name]
	if !known {
		reply.WriteError(unknownCommandMessage(args))
		return
	}
	if !arityMatches(command.arity, len(args)) {
		reply.WriteError(fmt.Sprintf("ERR wrong number of arguments for '%s' command", name))
		return
	}
	command.run(keyspace, args[1:], reply)
}

func arityMatches(arity, argumentCount int) bool {
	if arity >= 0 {
		return argumentCount == arity
	}
	return argumentCount >= -arity
}

func unknownCommandMessage(args []string) string {
	var message strings.Builder
	fmt.Fprintf(&message, "ERR unknown command '%s'", args[0])
	if len(args) > 1 {
		message.WriteString(", with args beginning with: ")
	}
	for _, arg := range args[1:] {
		fmt.Fprintf(&message, "'%s' ", arg)
	}
	return message.String()
}

// writeOptionalValue replies with a value that may be absent, which Redis
// encodes as a null bulk string.
func writeOptionalValue(reply *resp.Writer, value string, found bool, err error) {
	switch {
	case err != nil:
		reply.WriteError(err.Error())
	case !found:
		reply.WriteNullBulkString()
	default:
		reply.WriteBulkString(value)
	}
}
