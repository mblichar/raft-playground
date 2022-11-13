package node

import (
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_state"
)

type raftCommandHandler interface {
	handleAppendEntries(
		state *raft_state.FullStateAccessor,
		command raft_commands.AppendEntriesCommand,
	) raft_commands.AppendEntriesResult
}
