package cli

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/mblichar/raft/src/client_networking"
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/logging"
	"github.com/rivo/tview"
	"strconv"
	"strings"
)

func listenForUserCommands(inputField *tview.InputField, context *appContext, quit chan struct{}) {
	logger := logging.CreateLogger("[green][COMMAND[]", context.logs)
	commandsChannel := make(chan string)
	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			command := inputField.GetText()
			if len(command) > 0 {
				commandsChannel <- command
			}
		}
	})

	for {
		select {
		case command := <-commandsChannel:
			handleCommand(command, context, logger)
			inputField.SetText("")
		case <-quit:
			return
		}
	}
}

func handleCommand(command string, context *appContext, logger *logging.Logger) {
	tokens := strings.Split(command, " ")
	switch tokens[0] {
	case "client":
		if len(tokens) < 4 {
			logInvalidCommand(command, logger)
			return
		}

		nodeId, err := strconv.Atoi(tokens[1])
		if err != nil {
			logInvalidCommand(command, logger)
			return
		}

		if channel, ok := context.networkController.clientListenChannels[uint(nodeId)]; ok {
			result := make(chan client_networking.CommandResult)
			commandWrapper := client_networking.CommandWrapper{
				Command: strings.Join(tokens[2:], " "),
				Result:  result,
			}
			logger.Log(command)
			channel <- commandWrapper
			go func() {
				r := <-result
				logger.Log(fmt.Sprintf("'%s' result - success: %t, result: %s", command, r.Success, r.Result))
			}()
		} else {
			logInvalidCommand(command, logger)
		}
	case "node-restart":
		if len(tokens) != 2 {
			logInvalidCommand(command, logger)
			return
		}

		nodeId, err := strconv.Atoi(tokens[1])
		if err != nil {
			logInvalidCommand(command, logger)
			return
		}

		if node, ok := context.nodesById[uint(nodeId)]; ok {
			close(context.nodeQuitChannels[uint(nodeId)])
			node.RestartNode()
			startNode(uint(nodeId), node, context)
			logger.Log(command)
		} else {
			logInvalidCommand(command, logger)
		}
	case "network-splits":
		if len(tokens) < 2 {
			logInvalidCommand(command, logger)
			return
		}
		splits := make([][]uint, len(tokens[1:]))
		for i, token := range tokens[1:] {
			nodes := strings.Split(token, ",")
			splits[i] = make([]uint, len(nodes))

			for j, nodeIdStr := range nodes {
				if nodeId, err := strconv.Atoi(nodeIdStr); err == nil {
					splits[i][j] = uint(nodeId)
				} else {
					logInvalidCommand(command, logger)
					return
				}
			}
		}

		logger.Log(command)
		context.networkController.networkSplits = splits
	case "network-latency", "election-timeout", "heartbeat-timeout", "retry-timeout":
		if len(tokens) != 2 {
			logInvalidCommand(command, logger)
			return
		}

		if timeout, err := strconv.Atoi(tokens[1]); err == nil {
			switch tokens[0] {
			case "network-latency":
				config.Config.NetworkLatency = timeout
			case "election-timeout":
				config.Config.ElectionTimeout = timeout
			case "heartbeat-timeout":
				config.Config.HeartbeatTimeout = timeout
			case "retry-timeout":
				config.Config.RetryTimeout = timeout
			}
			logger.Log(command)
		} else {
			logInvalidCommand(command, logger)
		}
	case "help":
		logHelp(logger)
	default:
		logInvalidCommand(command, logger)
	}

}

func logInvalidCommand(command string, logger *logging.Logger) {
	logger.Log(fmt.Sprintf("'%s' - invalid command", command))
	logHelp(logger)
}

func logHelp(logger *logging.Logger) {
	logger.LogMultiple([]string{
		"Available commands:",
		"client [NODE_ID[] [COMMAND[] (e.g. client 2 set x 3) - sends client command to given node",
		"node-restart [NODE_ID[] (e.g. node-restart 2) - restarts given node (clears volatile state)",
		"network-latency[TIMEOUT[] (e.g. network-latency 2000) - sets network latency (in milliseconds)",
		"network-splits [SPLITS[] (e.g network-splits 1,2,3 4,5) - splits nodes into sets that can communicate only",
		"                        with other nodes in the same set. Use 'network-split 1,2,3,4,5' to enable communication between all nodes",
		"election-timeout [TIMEOUT[] (e.g. election-timeout 2000) - sets election timeout (in milliseconds)",
		"heartbeat-timeout [TIMEOUT[] (e.g. heartbeat-timeout 2000) - sets heartbeat timeout (in milliseconds)",
		"retry-timeout [TIMEOUT[] (e.g. retry-timeout 2000) - sets retry timeout (in milliseconds)",
		"help - displays this information",
	})
}
