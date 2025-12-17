package main

import (
	"bufio"
	"fmt"
	"net"
	"os"

	"github.com/Shub3am/open-redis/parser"
)

// Ensures gofmt doesn't remove the "net" and "os" imports in stage 1 (feel free to remove this!)
var _ = net.Listen
var _ = os.Exit

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.

	temp_mem := map[string]string{}
	fmt.Println("Logs from your program will appear here!")

	// Uncomment the code below to pass the first stage
	port := 6379
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		fmt.Printf("Failed to bind to port %d", port)
		os.Exit(1)
	}
	fmt.Printf("Redis Started at 0.0.0.0:%d", port)

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConn(conn, temp_mem)

	}
}

func handleConn(conn net.Conn, memory map[string]string) {
	reader := bufio.NewReader(conn)
	for {
		parsed, err := parser.RESPReader(reader)
		if err != nil {
			conn.Close()
			return
		}
		result, err := parser.RESPParser(parsed)
		if err != nil {

			conn.Close()
			return
		}
		output, err := parser.Execute(result, memory)
		if err != nil {

			conn.Close()
			return
		}

		conn.Write(output)

	}
}
