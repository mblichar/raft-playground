package node

import (
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/logger"
	"github.com/mblichar/raft/src/raft_networking"
	"github.com/mblichar/raft/src/raft_state"
	"github.com/mblichar/raft/src/timer"
)

type Node struct {
	VolatileState            raft_state.VolatileState
	PersistentState          raft_state.PersistentState
	raftNetworking           raft_networking.RaftNetworking
	timer                    timer.Timer
	logger                   logger.Logger
	electionCancelledChannel chan interface{}
}

func CreateNode(raftNetworking raft_networking.RaftNetworking, timer timer.Timer) Node {
	node := Node{
		raftNetworking: raftNetworking,
		timer:          timer,
	}

	// TODO: add init entry to log in PersistentState

	node.VolatileState.Role = raft_state.Follower

	return node
}

func (node *Node) Start(quit chan int) {
	raftCommandsChannel := node.raftNetworking.ListenForCommands()
	electionWonChannel := make(chan bool)

	for {
		var timeout timer.Timeout
		if node.VolatileState.Role == raft_state.Leader {
			timeout = node.timer.Timeout("heartbeat", config.Config.HeartbeatTimeout)
		} else {
			timeout = node.timer.Timeout("election", config.Config.ElectionTimeout) // TODO: add randomness
		}

		select {
		case commandWrapper := <-raftCommandsChannel:
			commandWrapper.Result <- handleRaftCommand(node, commandWrapper.Command)
		case <-electionWonChannel:
			node.VolatileState.Role = raft_state.Leader
			sendHeartbeat(node)
		case <-timeout.Done():
			if node.VolatileState.Role == raft_state.Leader {
				sendHeartbeat(node)
			} else if node.PersistentState.VotedFor == raft_state.NotVoted {
				electionWonChannel = startNewElection(node)
			}
		case <-quit:
			timeout.Cancel()
			return
		}
	}
}

func sendHeartbeat(node *Node) {
	// TODO: implement
}
