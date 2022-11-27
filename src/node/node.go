package node

import (
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_networking"
	"github.com/mblichar/raft/src/raft_state"
	"github.com/mblichar/raft/src/timer"
)

type RaftNode struct {
	VolatileState   raft_state.VolatileState
	PersistentState raft_state.PersistentState
	raftNetworking  raft_networking.RaftNetworking
	timer           timer.Timer
}

func CreateNode(raftNetworking raft_networking.RaftNetworking, timer timer.Timer) RaftNode {
	node := RaftNode{
		raftNetworking: raftNetworking,
		timer:          timer,
	}

	node.VolatileState.Role = raft_state.Follower

	return node
}

func (node *RaftNode) Start(quit chan int) {
	raftCommandsChannel := node.raftNetworking.ListenForCommands()

	for {
		var timeout timer.Timeout
		if node.VolatileState.Role == raft_state.Leader {
			timeout = node.timer.Timeout("heartbeat", config.Config.HeartbeatTimeout)
		} else {
			timeout = node.timer.Timeout("election", config.Config.ElectionTimeout)
		}

		select {
		case raftCommand := <-raftCommandsChannel:
			switch raftCommand.Command.CommandType() {
			case raft_commands.AppendEntries:
				raftCommand.Result <- node.handleAppendEntries(raftCommand.Command.ToAppendEntries())
			case raft_commands.RequestVote:
				raftCommand.Result <- node.handleRequestVote(raftCommand.Command.ToRequestVote())
			}
		case <-timeout.Done():
			if node.VolatileState.Role == raft_state.Leader {
				node.sendHeartbeat()
			} else {
				node.startElection()
			}
		case <-quit:
			timeout.Cancel()
			return
		}
	}
}

func (node *RaftNode) sendHeartbeat() {
	// TODO: implement
}

func (node *RaftNode) startElection() {
	node.VolatileState.Role = raft_state.Candidate
	node.PersistentState.CurrentTerm++
	node.PersistentState.VotedFor = node.PersistentState.NodeId
	requiredVotes := len(config.Config.RaftNodesIds)/2 + 1
	receivedVotes := 1 // candidate voted for itself

	results := node.sendRequestVoteCommands()
	timeout := node.timer.Timeout("election", config.Config.ElectionTimeout)

	for receivedVotes < requiredVotes {
		select {
		case result := <-results:
			if result.VoteGranted {
				receivedVotes++
			}
		case <-timeout.Done():
			return
		}
	}

	timeout.Cancel()

	// election won
	node.VolatileState.Role = raft_state.Leader
	node.sendHeartbeat()
}

func (node *RaftNode) sendRequestVoteCommands() <-chan raft_commands.RequestVoteResult {
	currentTerm := node.PersistentState.CurrentTerm
	currentNodeId := node.PersistentState.NodeId
	nodesCount := len(config.Config.RaftNodesIds)

	var lastLogIndex, lastLogTerm uint
	if len(node.PersistentState.Log) > 0 {
		lastLogEntry := node.PersistentState.Log[len(node.PersistentState.Log)-1]
		lastLogIndex = lastLogEntry.Index
		lastLogTerm = lastLogEntry.Term
	}

	// create buffered channel to ensure all goroutines spawned below don't leak and are able to put result in a channel
	// caller may not wait for all of the results because it needs only a majority of votes or may timeout
	results := make(chan raft_commands.RequestVoteResult, nodesCount-1)

	for i := 0; i < nodesCount; i++ {
		if nodeId := config.Config.RaftNodesIds[i]; nodeId != currentNodeId {
			go func() {
				result, err := node.raftNetworking.SendRequestVoteCommand(nodeId, raft_commands.RequestVoteCommand{
					CandidateId:  currentNodeId,
					Term:         currentTerm,
					LastLogIndex: lastLogIndex,
					LastLogTerm:  lastLogTerm,
				})

				if !err {
					results <- result
				}
			}()
		}
	}

	return results
}

func (node *RaftNode) handleAppendEntries(command *raft_commands.AppendEntriesCommand) *raft_commands.AppendEntriesResult {
	// TODO: implement
	return nil
}

func (node *RaftNode) handleRequestVote(command *raft_commands.RequestVoteCommand) *raft_commands.RequestVoteResult {
	// TODO: implement
	return nil
}
