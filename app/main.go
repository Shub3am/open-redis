package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"
)

var _ = net.Listen
var _ = os.Exit

type record struct {
	value     string
	expiresAt *time.Time
}

func main() {

	temp_mem := map[string]record{}
	fmt.Println("Logs from your program will appear here!")

	// Uncomment the code below to pass the first stage
	port := 6380
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

func handleConn(conn net.Conn, memory map[string]record) {
	reader := bufio.NewReader(conn)
	for {
		parsed, err := RESPReader(reader)
		if err != nil {
			conn.Close()
			return
		}
		result, err := RESPParser(parsed)
		if err != nil {

			conn.Close()
			return
		}
		output, err := Execute(result, memory)
		if err != nil {

			conn.Close()
			return
		}

		conn.Write(output)

	}
}
