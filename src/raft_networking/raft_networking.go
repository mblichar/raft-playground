package raft_networking

import "github.com/mblichar/raft-playground/src/raft_commands"

type CommandWrapper struct {
	Command raft_commands.RaftCommand
	Result  chan<- raft_commands.RaftCommandResult
}

type RaftNetworking interface {
	ListenForRaftCommands() chan CommandWrapper
	SendAppendEntriesCommand(nodeId uint, command raft_commands.AppendEntriesCommand) (raft_commands.RaftCommandResult, bool)
	SendRequestVoteCommand(nodeId uint, command raft_commands.RequestVoteCommand) (raft_commands.RaftCommandResult, bool)
}
