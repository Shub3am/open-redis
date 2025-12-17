package main

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Assuming it is always an array
func RESPReader(r *bufio.Reader) ([]byte, error) {
	var tokens []byte
	header, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	newHeader := strings.Split(string(header), "\r\n")
	headerLength, err := strconv.Atoi(newHeader[0][1:])
	tokens = append(tokens, []byte(header)...)
	if err != nil {
		return nil, err
	}
	shouldRun := true
	linesRead := 0
	for shouldRun {
		receiver, err := r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		strBuf := string(receiver)

		if strings.HasSuffix(string(strBuf), "\r\n") {
			tokens = append(tokens, receiver...)
			linesRead++
		} else {
			return nil, errors.New("corrupted Buffer")
		}
		if headerLength == linesRead/2 {
			break
		}
	}
	return tokens, nil
}

func RESPParser(raw []byte) ([]string, error) {
	commands := strings.Split(string(raw), "\r\n")
	validateString := false
	lengthToValidate := 0
	parsedCommands := []string{}
	fmt.Println(commands, "commands")
	for x := 1; x < len(commands)-1; x++ {
		currItem := commands[x]
		if currItem == "" {
			continue
		}
		if validateString {
			if len(currItem) == lengthToValidate {
				parsedCommands = append(parsedCommands, currItem)
				validateString = false
			} else {
				return nil, errors.New("corrupted buffer")
			}
		} else {
			if currItem[0] == '$' {
				validateString = true
				lenOfNextItem, err := strconv.Atoi(currItem[1:])
				if err != nil {
					return nil, err
				}
				lengthToValidate = lenOfNextItem
			}
		}
	}
	fmt.Println(parsedCommands, "Parsed Commands")
	return parsedCommands, nil
}

func Execute(parsed []string, memory map[string]record) ([]byte, error) {
	if len(parsed) == 0 {
		return nil, errors.New("empty command")
	}

	cmd := strings.ToLower(parsed[0])
	fmt.Println(cmd)
	switch cmd {
	case "ping":
		return []byte(("+PONG\r\n")), nil
	case "set":
		if len(parsed) < 3 {
			return []byte("+missing set values\r\n"), nil
		}
		// 0 1 2 3 4 5 [set key value arg value]
		new_record := record{value: parsed[2], expiresAt: nil}
		fmt.Println(parsed, len(parsed))
		if len(parsed[3:]) <= 2 {
			fmt.Println(parsed[3:], parsed[3])
			switch strings.ToLower(parsed[3]) {
			case "ex":
				expiryTime, err := strconv.Atoi(parsed[4])
				if err != nil {
					return []byte("+unable to parse expire\r\n"), nil
				}
				t := time.Now().Add(time.Duration(expiryTime) * time.Second)
				new_record.expiresAt = &t

			case "px":
				expiryTime, err := strconv.Atoi(parsed[4])
				if err != nil {
					return []byte("+unable to parse expire\r\n"), nil
				}
				t := time.Now().Add(time.Duration(expiryTime) * time.Millisecond)
				new_record.expiresAt = &t
			}

		}
		memory[parsed[1]] = new_record

		return []byte("+OK\r\n"), nil
	case "get":
		if len(parsed) < 2 {
			return []byte("+missing key for get command\r\n"), nil
		}
		property, ok := memory[parsed[1]]
		if !ok {
			return []byte("$-1\r\n"), nil
		}
		if property.expiresAt != nil && time.Now().After(*property.expiresAt) {
			delete(memory, parsed[1])
			return []byte("$-1\r\n"), nil

		}
		return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(property.value), property.value)), nil

	case "echo":
		if len(parsed) < 2 {
			return []byte("+missing echo value\r\n"), nil
		}
		return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(parsed[1]), parsed[1])), nil

	}

	return []byte("+unsupported command\r\n"), nil

}

// // ONLY FOR TESTING WHILE DEBUGGING
// func main() {
// 	stored, err := RESPParser([]byte("*2\r\n$0\r\n\r\n$3\r\nhey\r\n"))
// 	fmt.Println(stored, err)
// }
