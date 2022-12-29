package node

import (
	"fmt"
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_state"
)

func handleRaftCommand(node *Node, command raft_commands.RaftCommand) raft_commands.RaftCommandResult {
	commandTerm := command.CommandTerm()
	currentTerm := node.PersistentState.CurrentTerm

	if commandTerm < currentTerm {
		return raft_commands.RaftCommandResult{Term: node.PersistentState.CurrentTerm, Success: false}
	}

	if commandTerm > currentTerm {
		cancelElection(node)
		node.PersistentState.VotedFor = raft_state.NotVoted
		node.VolatileState.Role = raft_state.Follower
		node.PersistentState.CurrentTerm = commandTerm
	}

	switch command.CommandType() {
	case raft_commands.AppendEntries:
		return handleAppendEntries(node, command.ToAppendEntries())
	case raft_commands.RequestVote:
		return handleRequestVote(node, command.ToRequestVote())
	}

	panic(any(fmt.Sprintf("Received unsupported command: %d", command.CommandType())))
}

func handleRequestVote(
	node *Node,
	command *raft_commands.RequestVoteCommand,
) raft_commands.RaftCommandResult {
	persistentState := &node.PersistentState
	result := raft_commands.RaftCommandResult{Term: persistentState.CurrentTerm, Success: false}

	if persistentState.VotedFor != raft_state.NotVoted && uint(persistentState.VotedFor) != command.CandidateId {
		return result
	}

	lastLogEntry := persistentState.Log[len(persistentState.Log)-1]

	if lastLogEntry.Term <= command.LastLogTerm && lastLogEntry.Index <= command.LastLogIndex {
		persistentState.VotedFor = int(command.CandidateId)
		result.Success = true
		return result
	}

	return result
}

func handleAppendEntries(
	node *Node,
	command *raft_commands.AppendEntriesCommand,
) raft_commands.RaftCommandResult {
	persistentState := &node.PersistentState
	volatileState := &node.VolatileState
	result := raft_commands.RaftCommandResult{Term: persistentState.CurrentTerm}

	if volatileState.Role == raft_state.Leader {
		panic(any("Received append entries as leader with the same current term"))
	}

	if volatileState.Role == raft_state.Candidate {
		// it's the case in which leader with the same term was elected
		// case in which leader with higher term was elected is handled in generic handleRaftCommand
		cancelElection(node)
		volatileState.Role = raft_state.Follower
	}

	prevLogEntry, indexInLog := findEntryWithIndex(persistentState.Log, command.PrevLogIndex)
	if prevLogEntry == nil {
		return result
	}

	if prevLogEntry.Term != command.PrevLogTerm {
		return result
	}

	if len(command.Entries) > 0 {
		// advance indexInLog to next log entry after prevLogEntry
		indexInLog++
		for i := 0; i < len(command.Entries); i, indexInLog = i+1, indexInLog+1 {
			if indexInLog >= len(persistentState.Log) {
				persistentState.Log = append(persistentState.Log, command.Entries[i])
			} else if !entriesMatch(&persistentState.Log[indexInLog], &command.Entries[i]) {
				persistentState.Log = persistentState.Log[:indexInLog+1]
				persistentState.Log[indexInLog] = command.Entries[i]
			}
		}

		if command.LeaderCommitIndex > volatileState.CommitIndex {
			lastEntry := command.Entries[len(command.Entries)-1]
			if command.LeaderCommitIndex < lastEntry.Index {
				volatileState.CommitIndex = command.LeaderCommitIndex
			} else {
				volatileState.CommitIndex = lastEntry.Index
			}
		}
	}

	persistentState.CurrentTerm = command.Term
	volatileState.LeaderId = command.LeaderId
	result.Success = true
	result.Term = persistentState.CurrentTerm
	return result
}

func findEntryWithIndex(entries []raft_state.LogEntry, index uint) (*raft_state.LogEntry, int) {
	if len(entries) == 0 {
		return nil, -1
	}

	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Index == index {
			return &entries[i], i
		}

		// log entries are naturally sorted by index
		if entries[i].Index < index {
			return nil, -1
		}
	}

	return nil, -1
}

func entriesMatch(a *raft_state.LogEntry, b *raft_state.LogEntry) bool {
	return a.Index == b.Index && a.Term == b.Term
}
