package node

import (
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/raft_networking"
	"github.com/mblichar/raft/src/raft_state"
	"github.com/mblichar/raft/src/timer"
)

type Node struct {
	VolatileState            raft_state.VolatileState
	PersistentState          raft_state.PersistentState
	electionResultChannel    chan electionResult
	electionCancelledChannel chan struct{}
}

func CreateNode() Node {
	node := Node{}

	// TODO: add init entry to log in PersistentState

	node.VolatileState.Role = raft_state.Follower

	return node
}

func ProcessingLoop(node *Node, raftNetworking raft_networking.RaftNetworking, timeoutFactory timer.TimeoutFactory, quit chan int) {
	raftCommandsChannel := raftNetworking.ListenForCommands()

	for {
		var timeout timer.Timeout
		if node.VolatileState.Role == raft_state.Leader {
			timeout = timeoutFactory.Timeout("heartbeat", config.Config.HeartbeatTimeout)
		} else {
			timeout = timeoutFactory.Timeout("election", config.Config.ElectionTimeout) // TODO: add randomness
		}

		select {
		case commandWrapper := <-raftCommandsChannel:
			commandWrapper.Result <- handleRaftCommand(node, commandWrapper.Command)
		case result := <-node.electionResultChannel:
			cancelElection(node)
			if result.won {
				node.VolatileState.Role = raft_state.Leader
				sendHeartbeat(node)
			} else {
				node.VolatileState.Role = raft_state.Follower
				node.PersistentState.CurrentTerm = result.currentTerm
			}
		case <-timeout.Done():
			if node.VolatileState.Role == raft_state.Leader {
				sendHeartbeat(node)
			} else if node.PersistentState.VotedFor == raft_state.NotVoted {
				startNewElection(node, raftNetworking, timeoutFactory)
			}
		case <-quit:
			return
		}
	}
}

func sendHeartbeat(node *Node) {
	// TODO: implement
}
