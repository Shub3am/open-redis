package command

import (
	"bytes"
	"testing"

	"github.com/Shub3am/open-redis/resp"
	"github.com/Shub3am/open-redis/store"
)

// run executes one command and returns the raw RESP reply.
func run(t *testing.T, keyspace *store.Store, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	reply := resp.NewWriter(&output)
	Execute(keyspace, args, reply)
	if err := reply.Flush(); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

type exchange struct {
	args []string
	want string
}

// runScript runs commands in order against one keyspace and checks each reply.
func runScript(t *testing.T, keyspace *store.Store, script []exchange) {
	t.Helper()
	for _, step := range script {
		if got := run(t, keyspace, step.args...); got != step.want {
			t.Errorf("%q: got %q, want %q", step.args, got, step.want)
		}
	}
}

func TestExecuteRejectsUnknownCommandAndBadArity(t *testing.T) {
	runScript(t, store.New(), []exchange{
		{[]string{"NOPE"}, "-ERR unknown command 'NOPE'\r\n"},
		{[]string{"nope", "a", "b"}, "-ERR unknown command 'nope', with args beginning with: 'a' 'b' \r\n"},
		{[]string{"GET"}, "-ERR wrong number of arguments for 'get' command\r\n"},
		{[]string{"GET", "a", "b"}, "-ERR wrong number of arguments for 'get' command\r\n"},
		{[]string{"SET", "a"}, "-ERR wrong number of arguments for 'set' command\r\n"},
	})
}

func TestCommandNamesAreCaseInsensitive(t *testing.T) {
	runScript(t, store.New(), []exchange{
		{[]string{"pInG"}, "+PONG\r\n"},
		{[]string{"set", "k", "v"}, "+OK\r\n"},
		{[]string{"GeT", "k"}, "$1\r\nv\r\n"},
	})
}
