package raft_commands

type CommandType int

const (
	AppendEntries CommandType = iota
	RequestVote
)

type RaftCommand interface {
	// CommandType returns type of given command
	CommandType() CommandType
	// CommandTypeString returns type of given command as string
	CommandTypeString() string
	// CommandTerm returns term of command sender
	CommandTerm() uint
	// ToAppendEntries returns pointer to AppendEntriesCommand or nil if given command is of other type
	ToAppendEntries() *AppendEntriesCommand
	// ToRequestVote returns pointer to RequestVoteCommand or nil if given command is of other type
	ToRequestVote() *RequestVoteCommand
}

type RaftCommandResult struct {
	// currentTerm of given follower, for leader to update itself
	Term uint
	// boolean indicating whether entry was appended
	Success bool
}
