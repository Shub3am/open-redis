package command

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Shub3am/open-redis/store"
)

func TestDelAndExists(t *testing.T) {
	keyspace := store.New()
	runScript(t, keyspace, []exchange{
		{[]string{"SET", "a", "1"}, "+OK\r\n"},
		{[]string{"SET", "b", "2"}, "+OK\r\n"},
		{[]string{"EXISTS", "a", "b", "a", "missing"}, ":3\r\n"},
		{[]string{"DEL", "a", "missing"}, ":1\r\n"},
		{[]string{"EXISTS", "a"}, ":0\r\n"},
		{[]string{"DEL"}, "-ERR wrong number of arguments for 'del' command\r\n"},
	})
}

func TestExpireTTLAndPTTL(t *testing.T) {
	keyspace := store.New()
	runScript(t, keyspace, []exchange{
		{[]string{"TTL", "missing"}, ":-2\r\n"},
		{[]string{"PTTL", "missing"}, ":-2\r\n"},
		{[]string{"EXPIRE", "missing", "10"}, ":0\r\n"},
		{[]string{"SET", "k", "v"}, "+OK\r\n"},
		{[]string{"TTL", "k"}, ":-1\r\n"},
		{[]string{"EXPIRE", "k", "100"}, ":1\r\n"},
		{[]string{"TTL", "k"}, ":100\r\n"},
		{[]string{"EXPIRE", "k", "soon"}, "-ERR value is not an integer or out of range\r\n"},
		{[]string{"EXPIRE", "k", "9223372036854775807"}, "-ERR invalid expire time in 'expire' command\r\n"},
		{[]string{"PEXPIRE", "k", "-1"}, ":1\r\n"},
		{[]string{"EXISTS", "k"}, ":0\r\n"},
	})

	run(t, keyspace, "SET", "short", "v", "PX", "5000")
	pttl := run(t, keyspace, "PTTL", "short")
	if !strings.HasPrefix(pttl, ":4") && pttl != ":5000\r\n" {
		t.Fatalf("PTTL right after PX 5000 = %q, want about 5000", pttl)
	}
}

func TestKeyExpiresThroughCommands(t *testing.T) {
	keyspace := store.New()
	runScript(t, keyspace, []exchange{
		{[]string{"SET", "k", "v", "PX", "20"}, "+OK\r\n"},
		{[]string{"GET", "k"}, "$1\r\nv\r\n"},
	})
	time.Sleep(30 * time.Millisecond)
	runScript(t, keyspace, []exchange{
		{[]string{"GET", "k"}, "$-1\r\n"},
		{[]string{"TTL", "k"}, ":-2\r\n"},
	})
}

func TestKeysTypeAndFlushAll(t *testing.T) {
	keyspace := store.New()
	runScript(t, keyspace, []exchange{
		{[]string{"KEYS", "*"}, "*0\r\n"},
		{[]string{"SET", "user:1", "a"}, "+OK\r\n"},
		{[]string{"SET", "user:2", "b"}, "+OK\r\n"},
		{[]string{"SET", "order:1", "c"}, "+OK\r\n"},
		{[]string{"KEYS", "order:*"}, "*1\r\n$7\r\norder:1\r\n"},
		{[]string{"TYPE", "user:1"}, "+string\r\n"},
		{[]string{"TYPE", "missing"}, "+none\r\n"},
	})

	matched := keyspace.Keys("user:?")
	slices.Sort(matched)
	if !slices.Equal(matched, []string{"user:1", "user:2"}) {
		t.Fatalf("got %q", matched)
	}

	runScript(t, keyspace, []exchange{
		{[]string{"FLUSHALL", "now"}, "-ERR syntax error\r\n"},
		{[]string{"FLUSHALL", "async"}, "+OK\r\n"},
		{[]string{"KEYS", "*"}, "*0\r\n"},
		{[]string{"SET", "k", "v"}, "+OK\r\n"},
		{[]string{"FLUSHALL"}, "+OK\r\n"},
		{[]string{"EXISTS", "k"}, ":0\r\n"},
	})
}
