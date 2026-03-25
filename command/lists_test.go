package command

import (
	"testing"

	"github.com/Shub3am/open-redis/store"
)

const wrongType = "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"

func TestListCommands(t *testing.T) {
	runScript(t, store.New(), []exchange{
		{[]string{"LLEN", "l"}, ":0\r\n"},
		{[]string{"LRANGE", "l", "0", "-1"}, "*0\r\n"},
		{[]string{"RPUSH", "l", "b", "c"}, ":2\r\n"},
		{[]string{"LPUSH", "l", "a", "z"}, ":4\r\n"},
		{[]string{"LRANGE", "l", "0", "-1"}, "*4\r\n$1\r\nz\r\n$1\r\na\r\n$1\r\nb\r\n$1\r\nc\r\n"},
		{[]string{"LRANGE", "l", "-2", "-1"}, "*2\r\n$1\r\nb\r\n$1\r\nc\r\n"},
		{[]string{"LRANGE", "l", "2", "1"}, "*0\r\n"},
		{[]string{"LRANGE", "l", "0", "x"}, "-ERR value is not an integer or out of range\r\n"},
		{[]string{"LLEN", "l"}, ":4\r\n"},
		{[]string{"LPOP", "l"}, "$1\r\nz\r\n"},
		{[]string{"RPOP", "l"}, "$1\r\nc\r\n"},
		{[]string{"RPOP", "l"}, "$1\r\nb\r\n"},
		{[]string{"LPOP", "l"}, "$1\r\na\r\n"},
		{[]string{"LPOP", "l"}, "$-1\r\n"},
		{[]string{"TYPE", "l"}, "+none\r\n"},
		{[]string{"LPUSH", "l"}, "-ERR wrong number of arguments for 'lpush' command\r\n"},
		{[]string{"RPUSH", "l"}, "-ERR wrong number of arguments for 'rpush' command\r\n"},
	})
}

func TestWrongTypeErrors(t *testing.T) {
	runScript(t, store.New(), []exchange{
		{[]string{"SET", "s", "v"}, "+OK\r\n"},
		{[]string{"RPUSH", "l", "a"}, ":1\r\n"},
		{[]string{"TYPE", "l"}, "+list\r\n"},
		{[]string{"LPUSH", "s", "a"}, wrongType},
		{[]string{"RPUSH", "s", "a"}, wrongType},
		{[]string{"LPOP", "s"}, wrongType},
		{[]string{"RPOP", "s"}, wrongType},
		{[]string{"LRANGE", "s", "0", "-1"}, wrongType},
		{[]string{"LLEN", "s"}, wrongType},
		{[]string{"GET", "l"}, wrongType},
		{[]string{"INCR", "l"}, wrongType},
		{[]string{"SET", "l", "now a string"}, "+OK\r\n"},
		{[]string{"GET", "l"}, "$12\r\nnow a string\r\n"},
	})
}
