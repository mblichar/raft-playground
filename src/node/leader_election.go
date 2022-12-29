package node

import (
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_state"
)

func startNewElection(node *Node) chan bool {
	cancelElection(node)
	electionCancelledChannel := make(chan interface{})
	node.electionCancelledChannel = electionCancelledChannel

	node.VolatileState.Role = raft_state.Candidate
	node.PersistentState.CurrentTerm++
	node.PersistentState.VotedFor = int(node.PersistentState.NodeId)

	requiredVotes := len(config.Config.RaftNodesIds)/2 + 1
	receivedVotes := 1 // candidate voted for itself

	results := sendRequestVoteCommands(node, electionCancelledChannel)

	electionWonChannel := make(chan bool)
	go func() {
		for receivedVotes < requiredVotes {
			select {
			case result := <-results:
				if result.Success {
					receivedVotes++
				}
			case <-electionCancelledChannel:
				return
			}
		}

		electionWonChannel <- true
	}()

	return electionWonChannel
}

func cancelElection(node *Node) {
	if node.electionCancelledChannel != nil {
		close(node.electionCancelledChannel)
		node.electionCancelledChannel = nil
	}
}

func sendRequestVoteCommands(node *Node, electionCancelledChannel chan interface{}) <-chan raft_commands.RaftCommandResult {
	currentTerm := node.PersistentState.CurrentTerm
	currentNodeId := node.PersistentState.NodeId
	nodesCount := len(config.Config.RaftNodesIds)

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

	for i := 0; i < nodesCount; i++ {
		if nodeId := config.Config.RaftNodesIds[i]; nodeId != currentNodeId {
			sendCommand := func() bool {
				result, err := node.raftNetworking.SendRequestVoteCommand(nodeId, command)
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

				timeout := node.timer.Timeout("request-vote-retry", config.Config.RetryTimeout)
				select {
				case <-timeout.Done():
					if sendCommand() {
						return
					}
				case <-electionCancelledChannel:
					return
				}
			}()
		}
	}

	return results
}
