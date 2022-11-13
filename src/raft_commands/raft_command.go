package raft_commands

type CommandType int

const (
	AppendEntries CommandType = iota
)

type RaftCommand struct {
	CommandType CommandType
}
