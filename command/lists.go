// This file holds the list commands. It must not touch string values.
package command

import (
	"github.com/Shub3am/open-redis/resp"
	"github.com/Shub3am/open-redis/store"
)

func lpush(keyspace *store.Store, args []string, reply *resp.Writer) {
	length, err := keyspace.PushFront(args[0], args[1:]...)
	writeLength(reply, length, err)
}

func rpush(keyspace *store.Store, args []string, reply *resp.Writer) {
	length, err := keyspace.PushBack(args[0], args[1:]...)
	writeLength(reply, length, err)
}

func llen(keyspace *store.Store, args []string, reply *resp.Writer) {
	length, err := keyspace.Len(args[0])
	writeLength(reply, length, err)
}

func lpop(keyspace *store.Store, args []string, reply *resp.Writer) {
	value, found, err := keyspace.PopFront(args[0])
	writeOptionalValue(reply, value, found, err)
}

func rpop(keyspace *store.Store, args []string, reply *resp.Writer) {
	value, found, err := keyspace.PopBack(args[0])
	writeOptionalValue(reply, value, found, err)
}

func lrange(keyspace *store.Store, args []string, reply *resp.Writer) {
	start, startOK := store.ParseInteger(args[1])
	stop, stopOK := store.ParseInteger(args[2])
	if !startOK || !stopOK {
		reply.WriteError(errNotInteger)
		return
	}
	selected, err := keyspace.Range(args[0], start, stop)
	if err != nil {
		reply.WriteError(err.Error())
		return
	}
	reply.WriteArray(selected)
}

func writeLength(reply *resp.Writer, length int, err error) {
	if err != nil {
		reply.WriteError(err.Error())
		return
	}
	reply.WriteInteger(int64(length))
}
