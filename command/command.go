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
	fmt.Fprintf(&message, "ERR unknown command '%s', with args beginning with: ", args[0])
	for _, arg := range args[1:] {
		fmt.Fprintf(&message, "'%s' ", arg)
	}
	return message.String()
}
