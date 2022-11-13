package raft_commands

import (
	"github.com/mblichar/raft/src/raft_state"
)

// AppendEntriesCommand is sent by leader to replicate log entries, also used as heartbeat
type AppendEntriesCommand struct {
	RaftCommand
	// Leader's term
	Term uint
	// Leader's id
	LeaderId uint
	// Index of log entry immediately preceding new ones
	PrevLogIndex uint
	// Term of PrevLogIndex entry
	PrevLogTerm uint
	// LogEntries to store (empty for heartbeat)
	Entries []raft_state.LogEntry
	// Leader's commit index
	LeaderCommitIndex uint
}

type AppendEntriesResult struct {
	// currentTerm of given follower, for leader to update itself
	Term uint
	// boolean indicating whether entry was appended
	Success bool
}
