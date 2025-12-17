package parser

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

func Execute(parsed []string) ([]byte, error) {
	if len(parsed) == 0 {
		return nil, errors.New("empty command")
	}
	cmd := strings.ToLower(parsed[0])
	if cmd == "ping" {
		return []byte(("+PONG\r\n")), nil
	}
	if cmd != "echo" {
		return nil, errors.New("unsupported command")
	}

	return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(parsed[1]), parsed[1])), nil

}

// // ONLY FOR TESTING WHILE DEBUGGING
// func main() {
// 	stored, err := RESPParser([]byte("*2\r\n$0\r\n\r\n$3\r\nhey\r\n"))
// 	fmt.Println(stored, err)
// }
