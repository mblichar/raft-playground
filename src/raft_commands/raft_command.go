package raft_commands

type CommandType int

const (
	AppendEntries CommandType = iota
	RequestVote
)

type RaftCommand interface {
	// CommandType returns type of given command
	CommandType() CommandType
	// CommandTerm returns term of command sender
	CommandTerm() uint
	// ToAppendEntries returns pointer to AppendEntriesCommand or nil if given command is of other type
	ToAppendEntries() *AppendEntriesCommand
	// ToRequestVote returns pointer to RequestVoteCommand or nil if given command is of other type
	ToRequestVote() *RequestVoteCommand
}

type RaftCommandResult interface {
	// CommandType returns type of command which produced a given result
	CommandType() CommandType
}
