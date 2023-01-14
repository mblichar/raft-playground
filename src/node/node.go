package node

import (
	"fmt"
	"github.com/mblichar/raft-playground/src/client_networking"
	"github.com/mblichar/raft-playground/src/config"
	"github.com/mblichar/raft-playground/src/logging"
	"github.com/mblichar/raft-playground/src/raft_networking"
	"github.com/mblichar/raft-playground/src/raft_state"
	"github.com/mblichar/raft-playground/src/timer"
	"math/rand"
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

func CreateNode(nodeId uint) *Node {
	node := Node{}

	node.PersistentState.NodeId = nodeId
	node.PersistentState.VotedFor = raft_state.NotVoted
	node.PersistentState.Log = []raft_state.LogEntry{{Index: 0, Term: 0, Command: ""}}
	node.VolatileState.Role = raft_state.Follower
	node.ApplicationDatabase = make(map[string]string)

	return &node
}

func (node *Node) RestartNode() {
	node.stateMutex.Lock()
	node.VolatileState = raft_state.VolatileState{
		Role: raft_state.Follower,
	}
	node.ApplicationDatabase = make(map[string]string)
	node.stateMutex.Unlock()
}

func StartProcessingLoop(
	node *Node,
	raftNetworking raft_networking.RaftNetworking,
	clientNetworking client_networking.ClientNetworking,
	timeoutFactory timer.TimeoutFactory,
	logger *logging.Logger,
	quit chan struct{},
) {
	raftCommandsChannel := raftNetworking.ListenForRaftCommands()
	clientCommandsChannel := clientNetworking.ListenForClientCommands()

	for {
		var timeout timer.Timeout
		if node.VolatileState.Role == raft_state.Leader {
			timeout = timeoutFactory.Timeout("heartbeat", config.Config.HeartbeatTimeout)
		} else {
			timeout = timeoutFactory.Timeout("election",
				config.Config.ElectionTimeout+rand.Intn(int(config.Config.ElectionTimeout)/2))
		}

		select {
		case commandWrapper := <-clientCommandsChannel:
			logger.Log("Received client command")
			handleClientCommand(node, raftNetworking, clientNetworking, timeoutFactory, commandWrapper)
		case commandWrapper := <-raftCommandsChannel:
			logger.Log(fmt.Sprintf("Received %s command", commandWrapper.Command.CommandTypeString()))
			commandWrapper.Result <- handleRaftCommand(node, commandWrapper.Command)
		case result := <-node.electionResultChannel:
			node.stateMutex.Lock()
			cancelElection(node)
			if node.VolatileState.Role != raft_state.Candidate {
				// ignore election if state changed in some other way before getting it's result
				node.stateMutex.Unlock()
				continue
			}

			if result.won {
				logger.Log("Won election")
				if result.currentTerm == node.PersistentState.CurrentTerm {
					node.VolatileState.Role = raft_state.Leader
					node.VolatileState.LeaderId = node.PersistentState.NodeId
					go sendHeartbeat(node, raftNetworking, timeoutFactory)
				}
			} else {
				logger.Log("Lost election")
				node.VolatileState.Role = raft_state.Follower
				if result.currentTerm > node.PersistentState.CurrentTerm {
					node.PersistentState.VotedFor = raft_state.NotVoted
					node.PersistentState.CurrentTerm = result.currentTerm
				}
			}
			node.stateMutex.Unlock()
		case <-timeout.Done():
			if node.VolatileState.Role == raft_state.Leader {
				logger.Log("Heartbeat timeout")
				go sendHeartbeat(node, raftNetworking, timeoutFactory)
			} else {
				logger.Log("Election timeout")
				startNewElection(node, raftNetworking, timeoutFactory)
			}
		case <-quit:
			return
		}
	}
}
