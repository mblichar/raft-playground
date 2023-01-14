package raft_commands

import (
	"github.com/mblichar/raft-playground/src/raft_state"
)

// AppendEntriesCommand is sent by leader to replicate log entries, also used as heartbeat
type AppendEntriesCommand struct {
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

func (*AppendEntriesCommand) CommandType() CommandType {
	return AppendEntries
}

func (*AppendEntriesCommand) CommandTypeString() string {
	return "AppendEntries"
}

func (command *AppendEntriesCommand) CommandTerm() uint {
	return command.Term
}

func (command *AppendEntriesCommand) ToAppendEntries() *AppendEntriesCommand {
	return command
}

func (*AppendEntriesCommand) ToRequestVote() *RequestVoteCommand {
	return nil
}
