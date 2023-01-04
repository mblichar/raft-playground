package node

import (
	"github.com/mblichar/raft/src/client_networking"
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/raft_networking"
	"github.com/mblichar/raft/src/raft_state"
	"github.com/mblichar/raft/src/timer"
	"sync"
)

type Node struct {
	VolatileState            raft_state.VolatileState
	PersistentState          raft_state.PersistentState
	ApplicationDatabase      map[string]string
	electionResultChannel    chan electionResult
	electionCancelledChannel chan struct{}
	stateMutex               sync.Mutex
}

func CreateNode() *Node {
	node := Node{}

	// TODO: add init entry to log in PersistentState

	node.VolatileState.Role = raft_state.Follower
	node.ApplicationDatabase = make(map[string]string)

	return &node
}

func ProcessingLoop(
	node *Node,
	raftNetworking raft_networking.RaftNetworking,
	clientNetworking client_networking.ClientNetworking,
	timeoutFactory timer.TimeoutFactory,
	quit chan int,
) {
	raftCommandsChannel := raftNetworking.ListenForCommands()
	clientCommandsChannel := clientNetworking.ListenForCommands()

	for {
		var timeout timer.Timeout
		if node.VolatileState.Role == raft_state.Leader {
			timeout = timeoutFactory.Timeout("heartbeat", config.Config.HeartbeatTimeout)
		} else {
			timeout = timeoutFactory.Timeout("election", config.Config.ElectionTimeout) // TODO: add randomness
		}

		select {
		case commandWrapper := <-clientCommandsChannel:
			handleClientCommand(node, raftNetworking, clientNetworking, timeoutFactory, commandWrapper)
		case commandWrapper := <-raftCommandsChannel:
			commandWrapper.Result <- handleRaftCommand(node, commandWrapper.Command)
		case result := <-node.electionResultChannel:
			node.stateMutex.Lock()
			cancelElection(node)
			if node.VolatileState.Role != raft_state.Leader {
				// ignore election if state changed in some other way before getting it's result
				node.stateMutex.Unlock()
				continue
			}

			if result.won {
				if result.currentTerm == node.PersistentState.CurrentTerm {
					node.VolatileState.Role = raft_state.Leader
					go sendHeartbeat(node, raftNetworking, timeoutFactory)
				}
			} else {
				node.VolatileState.Role = raft_state.Follower
				if result.currentTerm > node.PersistentState.CurrentTerm {
					node.PersistentState.VotedFor = raft_state.NotVoted
					node.PersistentState.CurrentTerm = result.currentTerm
				}
			}
			node.stateMutex.Unlock()
		case <-timeout.Done():
			if node.VolatileState.Role == raft_state.Leader {
				go sendHeartbeat(node, raftNetworking, timeoutFactory)
			} else if node.PersistentState.VotedFor == raft_state.NotVoted {
				startNewElection(node, raftNetworking, timeoutFactory)
			}
		case <-quit:
			return
		}
	}
}
