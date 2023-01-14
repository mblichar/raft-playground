package node

import (
	"fmt"
	"github.com/mblichar/raft-playground/src/client_networking"
	"github.com/mblichar/raft-playground/src/config"
	"github.com/mblichar/raft-playground/src/raft_commands"
	"github.com/mblichar/raft-playground/src/raft_networking"
	"github.com/mblichar/raft-playground/src/raft_state"
	"github.com/mblichar/raft-playground/src/timer"
)

func handleClientCommand(
	node *Node,
	raftNetworking raft_networking.RaftNetworking,
	clientNetworking client_networking.ClientNetworking,
	timeoutFactory timer.TimeoutFactory,
	commandWrapper client_networking.CommandWrapper,
) {
	command := commandWrapper.Command
	if !isValidCommand(command) {
		commandWrapper.Result <- client_networking.CommandResult{
			Result:  fmt.Sprintf("'%s' - invalid command", command),
			Success: false,
		}
		return
	}

	if node.VolatileState.Role != raft_state.Leader {
		go func() {
			result, err := clientNetworking.SendCommand(node.VolatileState.LeaderId, command)
			if !err {
				commandWrapper.Result <- result
			} else {
				commandWrapper.Result <- client_networking.CommandResult{Result: "Failed to redirect to leader", Success: false}
			}
		}()
		return
	}

	if isReadOnlyCommand(command) {
		go handleReadOnlyClientCommand(node, raftNetworking, timeoutFactory, commandWrapper)
	} else {
		handleWriteClientCommand(node, raftNetworking, timeoutFactory, commandWrapper)
	}
}

func handleReadOnlyClientCommand(
	node *Node,
	raftNetworking raft_networking.RaftNetworking,
	timeoutFactory timer.TimeoutFactory,
	commandWrapper client_networking.CommandWrapper,
) {
	// check if still a leader
	if sendHeartbeat(node, raftNetworking, timeoutFactory) {
		commandWrapper.Result <- client_networking.CommandResult{Result: executeReadOnlyCommand(node, commandWrapper.Command), Success: true}
	} else {
		commandWrapper.Result <- client_networking.CommandResult{Result: "No longer active leader", Success: false}
	}
}

func handleWriteClientCommand(
	node *Node,
	raftNetworking raft_networking.RaftNetworking,
	timeoutFactory timer.TimeoutFactory,
	commandWrapper client_networking.CommandWrapper,
) {
	node.stateMutex.Lock()
	appendEntriesCommand := createAppendEntriesCommand(node)
	newLogEntry := raft_state.LogEntry{
		Term:    node.PersistentState.CurrentTerm,
		Command: commandWrapper.Command,
		Index:   appendEntriesCommand.PrevLogIndex + 1,
	}
	appendEntriesCommand.Entries = []raft_state.LogEntry{newLogEntry}
	node.PersistentState.Log = append(node.PersistentState.Log, newLogEntry)
	node.stateMutex.Unlock()

	go func() {
		commandReplicated := replicateLogEntry(node, raftNetworking, timeoutFactory, appendEntriesCommand)
		if commandReplicated {
			// apply all not-applied entries up to the one that was just replicated
			node.stateMutex.Lock()
			applyLogEntries(node, node.VolatileState.LastApplied, newLogEntry.Index)

			node.VolatileState.CommitIndex = newLogEntry.Index
			node.VolatileState.LastApplied = newLogEntry.Index
			node.stateMutex.Unlock()
			commandWrapper.Result <- client_networking.CommandResult{Result: "DONE", Success: true}
		} else {
			commandWrapper.Result <- client_networking.CommandResult{Result: "Failed to replicate command", Success: false}
		}
	}()
}

// sendHeartbeat returns boolean indicating whether heartbeat was received by majority of nodes
func sendHeartbeat(
	node *Node,
	raftNetworking raft_networking.RaftNetworking,
	timeoutFactory timer.TimeoutFactory,
) bool {
	node.stateMutex.Lock()
	if node.VolatileState.Role != raft_state.Leader {
		// only leader can send heartbeat
		node.stateMutex.Unlock()
		return false
	}

	command := createAppendEntriesCommand(node)
	command.Entries = []raft_state.LogEntry{}
	node.stateMutex.Unlock()

	return replicateLogEntry(node, raftNetworking, timeoutFactory, command)
}

func replicateLogEntry(
	node *Node,
	raftNetworking raft_networking.RaftNetworking,
	timeoutFactory timer.TimeoutFactory,
	appendEntriesCommand raft_commands.AppendEntriesCommand,
) bool {
	nodesCount := len(config.Config.NodeIds)
	requiredReplications := nodesCount/2 + 1
	replications := 1

	// buffered channel with len equal to nodes count - 1 to avoid goroutine leaks
	results := make(chan bool, len(config.Config.NodeIds)-1)
	for _, nodeId := range config.Config.NodeIds {
		if nodeId != node.PersistentState.NodeId {
			receiverId := nodeId // create local copy for goroutine
			go func() {
				results <- sendAppendEntriesCommand(
					node,
					receiverId,
					raftNetworking,
					timeoutFactory,
					appendEntriesCommand,
				)
			}()
		}
	}

	responsesCount := 0
	for {
		result := <-results
		if result {
			replications++
		}
		responsesCount++

		if replications >= requiredReplications {
			return true
		}
		if responsesCount == nodesCount-1 {
			return false
		}
	}
}

func sendAppendEntriesCommand(
	node *Node,
	receivingNodeId uint,
	raftNetworking raft_networking.RaftNetworking,
	timeoutFactory timer.TimeoutFactory,
	command raft_commands.AppendEntriesCommand,
) bool {
	currentTerm := command.Term

	shouldStopRetrying := func(shouldLock bool) bool {
		if shouldLock {
			node.stateMutex.Lock()
			defer node.stateMutex.Unlock()
		}
		// stop retrying when node is no longer a leader in command's term
		return node.VolatileState.Role != raft_state.Leader || currentTerm != node.PersistentState.CurrentTerm
	}
	for {
		if shouldStopRetrying(true) {
			return false
		}

		result, err := raftNetworking.SendAppendEntriesCommand(receivingNodeId, command)
		if err {
			<-timeoutFactory.Timeout("append-entries-retry", config.Config.RetryTimeout).Done()
		} else {
			if result.Success {
				return true
			}

			if result.Term > currentTerm {
				// Ignore converting to follower in that case for simplicity, AppendEntriesCommands are retried
				// indefinitely in parallel, so this may occur at any moment.
				// We should get heartbeat from a new leader and that would convert this node to follower
				return false
			}

			node.stateMutex.Lock()
			if shouldStopRetrying(false) {
				node.stateMutex.Unlock()
				return true
			}

			entryToInclude, err := findEntry(node.PersistentState.Log, command.PrevLogIndex)
			if err {
				// should not happen, all nodes share initial log entry
				panic(any("Should not happen - wasn't able to find entry to include"))
			}

			newPrevEntry, err := findPrevEntry(node.PersistentState.Log, command.PrevLogIndex)
			if err {
				// should not happen, all nodes share initial log entry
				panic(any("Should not happen - wasn't able to find new prev entry"))
			}

			command = raft_commands.AppendEntriesCommand{
				Term:              command.Term,
				LeaderId:          command.LeaderId,
				PrevLogIndex:      newPrevEntry.Index,
				PrevLogTerm:       newPrevEntry.Term,
				LeaderCommitIndex: command.LeaderCommitIndex,
				Entries:           append([]raft_state.LogEntry{entryToInclude}, command.Entries...),
			}
			node.stateMutex.Unlock()
		}
	}
}

func createAppendEntriesCommand(node *Node) raft_commands.AppendEntriesCommand {
	lastLogEntry := node.PersistentState.Log[len(node.PersistentState.Log)-1]
	return raft_commands.AppendEntriesCommand{
		Term:              node.PersistentState.CurrentTerm,
		LeaderId:          node.PersistentState.NodeId,
		PrevLogIndex:      lastLogEntry.Index,
		PrevLogTerm:       lastLogEntry.Term,
		LeaderCommitIndex: node.VolatileState.CommitIndex,
	}
}

func findPrevEntry(log []raft_state.LogEntry, index uint) (raft_state.LogEntry, bool) {
	for i := len(log) - 1; i > 0; i-- {
		if log[i].Index == index {
			return log[i-1], false
		}
	}

	return raft_state.LogEntry{}, true
}

func findEntry(log []raft_state.LogEntry, index uint) (raft_state.LogEntry, bool) {
	for i := len(log) - 1; i > 0; i-- {
		if log[i].Index == index {
			return log[i], false
		}
	}

	return raft_state.LogEntry{}, true
}
