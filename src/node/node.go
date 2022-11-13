package node

import "github.com/mblichar/raft/src/raft_state"

type RaftCommandsListener interface {
	Listen() chan interface{}
}

type RaftNode struct {
	raftCommandsListener RaftCommandsListener
	volatileState        raft_state.VolatileState
	persistentState      raft_state.PersistentState
}

func CreateNode(listener RaftCommandsListener) RaftNode {
	return RaftNode{
		raftCommandsListener: listener,
	}
}

func (node *RaftNode) Start(quit chan int) {
	raftCommandsChannel := node.raftCommandsListener.Listen()

	for {
		select {
		case raftCommand := <-raftCommandsChannel:
			node.handleRaftCommand(raftCommand)
		}
	}
}

func (node *RaftNode) handleRaftCommand(raftCommand interface{}) {

}
