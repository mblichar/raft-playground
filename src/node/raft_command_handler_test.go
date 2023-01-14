package node

import (
	"github.com/go-test/deep"
	"github.com/mblichar/raft-playground/src/raft_commands"
	"github.com/mblichar/raft-playground/src/raft_state"
	"testing"
)

func TestAppendEntriesHandling(t *testing.T) {
	createNode := func(term uint, commitIndex uint) *Node {
		return &Node{
			ApplicationDatabase: make(map[string]string),
			PersistentState: raft_state.PersistentState{
				CurrentTerm: term,
			},
			VolatileState: raft_state.VolatileState{
				CommitIndex: commitIndex,
				Role:        raft_state.Follower,
			},
		}
	}

	assertResult := func(t *testing.T, result raft_commands.RaftCommandResult, expectedSuccess bool, expectedTerm uint) {
		if result.Success != expectedSuccess {
			t.Errorf("expected success to be %t, got %t", expectedSuccess, result.Success)
		}

		if result.Term != expectedTerm {
			t.Errorf("expected result term to be %d, got %d", expectedTerm, result.Term)
		}
	}

	assertLogEntries := func(t *testing.T, logEntries []raft_state.LogEntry, expectedLogEntries []raft_state.LogEntry) {
		if diff := deep.Equal(logEntries, expectedLogEntries); diff != nil {
			t.Errorf("expected log entries to match, got the following differences %s", diff)
		}
	}

	t.Run("returns success: false when command term < state current term", func(t *testing.T) {
		node := createNode(2, 0)
		command := raft_commands.AppendEntriesCommand{Term: 1}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, false, 2)
	})

	t.Run("returns success: false when no log entry matching command prev log index", func(t *testing.T) {
		node := createNode(2, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 0},
			{Index: 1},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 2, Term: 2}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, false, 2)
	})

	t.Run("returns success: false when prev log index entry exist but with wrong term", func(t *testing.T) {
		node := createNode(2, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 0},
			{Index: 1, Term: 1},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 1, PrevLogTerm: 2, Term: 2}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, false, 2)
	})

	t.Run("appends new entries when prev entry matches", func(t *testing.T) {
		node := createNode(2, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 2, Command: "set b 1"},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 1, PrevLogTerm: 2, Term: 3, Entries: []raft_state.LogEntry{
			{Index: 2, Term: 3, Command: "set c 2"},
			{Index: 3, Term: 3, Command: "set d 3"},
		}}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 3)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 2, Command: "set b 1"},
			{Index: 2, Term: 3, Command: "set c 2"},
			{Index: 3, Term: 3, Command: "set d 3"},
		})
	})

	t.Run("appends only new entries", func(t *testing.T) {
		node := createNode(2, 3)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 2, Command: "set b 1"},
			{Index: 2, Term: 2, Command: "set c 2"},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 1, PrevLogTerm: 2, Term: 2, Entries: []raft_state.LogEntry{
			{Index: 2, Term: 2, Command: "set c 2"},
			{Index: 3, Term: 2, Command: "set d 3"},
		}}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 2)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 2, Command: "set b 1"},
			{Index: 2, Term: 2, Command: "set c 2"},
			{Index: 3, Term: 2, Command: "set d 3"},
		})
	})

	t.Run("removes conflicting entries", func(t *testing.T) {
		node := createNode(3, 1)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 3, Command: "set b 1"},
			{Index: 2, Term: 3, Command: "set c 2"},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 0, PrevLogTerm: 1, Term: 4, Entries: []raft_state.LogEntry{
			{Index: 2, Term: 4, Command: "set e 4"},
			{Index: 3, Term: 4, Command: "set f 5"},
		}}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 4)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 2, Term: 4, Command: "set e 4"},
			{Index: 3, Term: 4, Command: "set f 5"},
		})
	})

	t.Run("updates commit index to leader commit index when leader's committed entry in log", func(t *testing.T) {
		node := createNode(1, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 1, Command: "set b 1"},
		}

		leaderCommitIndex := uint(3)
		command := raft_commands.AppendEntriesCommand{
			PrevLogIndex:      1,
			PrevLogTerm:       1,
			Term:              2,
			LeaderCommitIndex: leaderCommitIndex,
			Entries: []raft_state.LogEntry{
				{Index: 2, Term: 2, Command: "set c 2"},
				{Index: 3, Term: 2, Command: "set d 3"},
			},
		}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 2)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 1, Command: "set b 1"},
			{Index: 2, Term: 2, Command: "set c 2"},
			{Index: 3, Term: 2, Command: "set d 3"},
		})
		if node.VolatileState.CommitIndex != leaderCommitIndex {
			t.Errorf("expected commit index to equal %d, got %d", leaderCommitIndex, node.VolatileState.CommitIndex)
		}
	})

	t.Run("updates commit index to last entry index when leader's committed entry not in log", func(t *testing.T) {
		node := createNode(1, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 1, Command: "set b 1"},
		}

		lastEntryIndex := uint(3)
		leaderCommitIndex := uint(4)
		command := raft_commands.AppendEntriesCommand{
			PrevLogIndex:      1,
			PrevLogTerm:       1,
			Term:              2,
			LeaderCommitIndex: leaderCommitIndex,
			Entries: []raft_state.LogEntry{
				{Index: 2, Term: 2, Command: "set c 2"},
				{Index: lastEntryIndex, Term: 2, Command: "set d 3"},
			},
		}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 2)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 0, Term: 1, Command: ""},
			{Index: 1, Term: 1, Command: "set b 1"},
			{Index: 2, Term: 2, Command: "set c 2"},
			{Index: lastEntryIndex, Term: 2, Command: "set d 3"},
		})
		if node.VolatileState.CommitIndex != lastEntryIndex {
			t.Errorf("expected commit index to equal %d, got %d", lastEntryIndex, node.VolatileState.CommitIndex)
		}
	})
}
