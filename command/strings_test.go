package command

import (
	"testing"

	"github.com/Shub3am/open-redis/store"
)

func TestPingAndEcho(t *testing.T) {
	runScript(t, store.New(), []exchange{
		{[]string{"PING"}, "+PONG\r\n"},
		{[]string{"PING", "hi"}, "$2\r\nhi\r\n"},
		{[]string{"PING", "a", "b"}, "-ERR wrong number of arguments for 'ping' command\r\n"},
		{[]string{"ECHO", "hello world"}, "$11\r\nhello world\r\n"},
		{[]string{"ECHO"}, "-ERR wrong number of arguments for 'echo' command\r\n"},
	})
}

func TestSetAndGet(t *testing.T) {
	runScript(t, store.New(), []exchange{
		{[]string{"GET", "k"}, "$-1\r\n"},
		{[]string{"SET", "k", "v"}, "+OK\r\n"},
		{[]string{"GET", "k"}, "$1\r\nv\r\n"},
		{[]string{"SET", "k", ""}, "+OK\r\n"},
		{[]string{"GET", "k"}, "$0\r\n\r\n"},
	})
}

func TestSetFlags(t *testing.T) {
	runScript(t, store.New(), []exchange{
		{[]string{"SET", "k", "v", "XX"}, "$-1\r\n"},
		{[]string{"SET", "k", "v", "NX"}, "+OK\r\n"},
		{[]string{"SET", "k", "v2", "nx"}, "$-1\r\n"},
		{[]string{"SET", "k", "v3", "xx", "ex", "100"}, "+OK\r\n"},
		{[]string{"GET", "k"}, "$2\r\nv3\r\n"},
		{[]string{"SET", "k", "v", "PX", "1500", "XX"}, "+OK\r\n"},
		{[]string{"SET", "k", "v", "NX", "XX"}, "-ERR syntax error\r\n"},
		{[]string{"SET", "k", "v", "EX", "1", "PX", "1"}, "-ERR syntax error\r\n"},
		{[]string{"SET", "k", "v", "EX"}, "-ERR syntax error\r\n"},
		{[]string{"SET", "k", "v", "BOGUS"}, "-ERR syntax error\r\n"},
		{[]string{"SET", "k", "v", "EX", "ten"}, "-ERR value is not an integer or out of range\r\n"},
		{[]string{"SET", "k", "v", "EX", "0"}, "-ERR invalid expire time in 'set' command\r\n"},
		{[]string{"SET", "k", "v", "PX", "-5"}, "-ERR invalid expire time in 'set' command\r\n"},
		{[]string{"SET", "k", "v", "EX", "9223372036854775807"}, "-ERR invalid expire time in 'set' command\r\n"},
	})
}

func TestIncrDecrIncrBy(t *testing.T) {
	runScript(t, store.New(), []exchange{
		{[]string{"INCR", "n"}, ":1\r\n"},
		{[]string{"INCR", "n"}, ":2\r\n"},
		{[]string{"DECR", "n"}, ":1\r\n"},
		{[]string{"DECR", "fresh"}, ":-1\r\n"},
		{[]string{"INCRBY", "n", "10"}, ":11\r\n"},
		{[]string{"INCRBY", "n", "-20"}, ":-9\r\n"},
		{[]string{"GET", "n"}, "$2\r\n-9\r\n"},
		{[]string{"INCRBY", "n", "1.5"}, "-ERR value is not an integer or out of range\r\n"},
		{[]string{"SET", "s", "abc"}, "+OK\r\n"},
		{[]string{"INCR", "s"}, "-ERR value is not an integer or out of range\r\n"},
		{[]string{"SET", "big", "9223372036854775807"}, "+OK\r\n"},
		{[]string{"INCR", "big"}, "-ERR increment or decrement would overflow\r\n"},
	})
}
