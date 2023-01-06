package raft_commands

type RequestVoteCommand struct {
	// Candidate's term
	Term uint
	// Id of candidate requesting vote
	CandidateId uint
	// Index of candidate's last log entry
	LastLogIndex uint
	// Term of candidate's last log entry
	LastLogTerm uint
}

func (*RequestVoteCommand) CommandType() CommandType {
	return RequestVote
}

func (*RequestVoteCommand) CommandTypeString() string {
	return "RequestVote"
}

func (command *RequestVoteCommand) CommandTerm() uint {
	return command.Term
}

func (*RequestVoteCommand) ToAppendEntries() *AppendEntriesCommand {
	return nil
}

func (command *RequestVoteCommand) ToRequestVote() *RequestVoteCommand {
	return command
}
