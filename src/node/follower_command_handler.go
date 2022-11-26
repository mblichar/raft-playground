package node

import (
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_state"
)

type followerCommandHandler struct{}

func (*followerCommandHandler) handleAppendEntries(
	state raft_state.FullStateAccessor,
	command raft_commands.AppendEntriesCommand,
) raft_commands.AppendEntriesResult {
	persistentState := state.PersistentState()
	volatileState := state.VolatileState()
	result := raft_commands.AppendEntriesResult{Term: persistentState.CurrentTerm}

	if command.Term < persistentState.CurrentTerm {
		return result
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
