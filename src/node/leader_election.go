package node

import (
	"github.com/mblichar/raft-playground/src/config"
	"github.com/mblichar/raft-playground/src/raft_commands"
	"github.com/mblichar/raft-playground/src/raft_networking"
	"github.com/mblichar/raft-playground/src/raft_state"
	"github.com/mblichar/raft-playground/src/timer"
)

type electionResult struct {
	// indicates whether node won the election or not
	won bool
	// when won equals false, indicates the current term received from other nodes
	currentTerm uint
}

func startNewElection(node *Node, raftNetworking raft_networking.RaftNetworking, timeoutFactory timer.TimeoutFactory) {
	node.stateMutex.Lock()
	cancelElection(node)
	electionResultChannel := make(chan electionResult)
	electionCancelledChannel := make(chan struct{})
	node.electionResultChannel = electionResultChannel
	node.electionCancelledChannel = electionCancelledChannel

	node.VolatileState.Role = raft_state.Candidate
	node.PersistentState.CurrentTerm++
	node.PersistentState.VotedFor = int(node.PersistentState.NodeId)

	requiredVotes := len(config.Config.NodeIds)/2 + 1
	receivedVotes := 1 // candidate voted for itself
	currentTerm := node.PersistentState.CurrentTerm

	results := sendRequestVoteCommands(node, electionCancelledChannel, raftNetworking, timeoutFactory)
	node.stateMutex.Unlock()

	go func() {
		for receivedVotes < requiredVotes {
			select {
			case result := <-results:
				if result.Success {
					receivedVotes++
				} else if result.Term > currentTerm {
					electionResultChannel <- electionResult{won: false, currentTerm: result.Term}
					return
				}
			case <-electionCancelledChannel:
				return
			}
		}

		electionResultChannel <- electionResult{won: true, currentTerm: currentTerm}
	}()
}

func cancelElection(node *Node) {
	if node.electionCancelledChannel != nil {
		close(node.electionCancelledChannel)
		node.electionCancelledChannel = nil
	}
}

func sendRequestVoteCommands(
	node *Node,
	electionCancelledChannel chan struct{},
	raftNetworking raft_networking.RaftNetworking,
	timeoutFactory timer.TimeoutFactory,
) <-chan raft_commands.RaftCommandResult {
	currentTerm := node.PersistentState.CurrentTerm
	currentNodeId := node.PersistentState.NodeId
	nodesCount := len(config.Config.NodeIds)

	var lastLogIndex, lastLogTerm uint
	lastLogEntry := node.PersistentState.Log[len(node.PersistentState.Log)-1]
	lastLogIndex = lastLogEntry.Index
	lastLogTerm = lastLogEntry.Term

	// create buffered channel to ensure all goroutines spawned below don't leak and are able to put result in a channel
	// caller may not wait for all of the results because it needs only a majority of votes or may timeout
	results := make(chan raft_commands.RaftCommandResult, nodesCount-1)
	command := raft_commands.RequestVoteCommand{
		CandidateId:  currentNodeId,
		Term:         currentTerm,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}

	for _, val := range config.Config.NodeIds {
		nodeId := val
		if nodeId != currentNodeId {
			sendCommand := func() bool {
				result, err := raftNetworking.SendRequestVoteCommand(nodeId, command)
				if !err {
					results <- result
					return true
				}

				return false
			}

			go func() {
				if sendCommand() {
					return
				}

				for {
					timeout := timeoutFactory.Timeout("request-vote-retry", config.Config.RetryTimeout)
					select {
					case <-timeout.Done():
						if sendCommand() {
							return
						}
					case <-electionCancelledChannel:
						return
					}
				}
			}()
		}
	}

	return results
}
