package node

import (
	"fmt"
	"strings"
)

func isValidCommand(command string) bool {
	tokens := strings.Split(command, " ")

	if len(tokens) <= 1 {
		return false
	}

	operation := strings.ToUpper(tokens[0])
	if operation == "GET" || operation == "DEL" {
		return len(tokens) == 2
	}

	if operation == "SET" {
		return len(tokens) == 3
	}

	return false
}

func isReadOnlyCommand(command string) bool {
	tokens := strings.Split(command, " ")
	return strings.ToUpper(tokens[0]) == "GET"
}

func executeReadOnlyCommand(node *Node, command string) string {
	tokens := strings.Split(command, " ")
	operation := strings.ToUpper(tokens[0])
	key := tokens[1]
	switch operation {
	case "GET":
		if val, ok := node.ApplicationDatabase[key]; ok {
			return val
		} else {
			return "NOT FOUND"
		}
	}
	panic(any(fmt.Sprintf("should not happen - unknown read only command: %s", command)))
}

func executeWriteCommand(node *Node, command string) {
	tokens := strings.Split(command, " ")
	operation := strings.ToUpper(tokens[0])
	key := tokens[1]
	switch operation {
	case "SET":
		node.ApplicationDatabase[key] = tokens[2]
	case "DEL":
		if _, ok := node.ApplicationDatabase[key]; ok {
			delete(node.ApplicationDatabase, key)
		}
	default:
		panic(any(fmt.Sprintf("should not happen - unknown write command: %s", command)))
	}
}
