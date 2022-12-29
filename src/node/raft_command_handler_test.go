package node

import (
	"github.com/go-test/deep"
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_state"
	"testing"
)

func TestAppendEntriesHandling(t *testing.T) {

	createNodeMock := func(term uint, commitIndex uint) *Node {
		return &Node{
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
			t.Fatalf("expected success to be %t, got %t", expectedSuccess, result.Success)
		}

		if result.Term != expectedTerm {
			t.Fatalf("expected result term to be %d, got %d", expectedTerm, result.Term)
		}
	}

	assertLogEntries := func(t *testing.T, logEntries []raft_state.LogEntry, expectedLogEntries []raft_state.LogEntry) {
		if diff := deep.Equal(logEntries, expectedLogEntries); diff != nil {
			t.Fatalf("expected log entries to match, got the following differences %s", diff)
		}
	}

	t.Run("returns success: false when command term < state current term", func(t *testing.T) {
		node := createNodeMock(2, 0)
		command := raft_commands.AppendEntriesCommand{Term: 1}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, false, 2)
	})

	t.Run("returns success: false when no log entry matching command prev log index", func(t *testing.T) {
		node := createNodeMock(2, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 1},
			{Index: 2},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 3, Term: 2}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, false, 2)
	})

	t.Run("returns success: false when prev log index entry exist but with wrong term", func(t *testing.T) {
		node := createNodeMock(2, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 1},
			{Index: 2, Term: 1},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 2, PrevLogTerm: 2, Term: 2}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, false, 2)
	})

	t.Run("appends new entries when prev entry matches", func(t *testing.T) {
		node := createNodeMock(2, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 2, Command: "b"},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 2, PrevLogTerm: 2, Term: 3, Entries: []raft_state.LogEntry{
			{Index: 3, Term: 3, Command: "c"},
			{Index: 4, Term: 3, Command: "d"},
		}}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 3)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 2, Command: "b"},
			{Index: 3, Term: 3, Command: "c"},
			{Index: 4, Term: 3, Command: "d"},
		})
	})

	t.Run("appends only new entries", func(t *testing.T) {
		node := createNodeMock(2, 3)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 2, Command: "b"},
			{Index: 3, Term: 2, Command: "c"},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 2, PrevLogTerm: 2, Term: 2, Entries: []raft_state.LogEntry{
			{Index: 3, Term: 2, Command: "c"},
			{Index: 4, Term: 2, Command: "d"},
		}}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 2)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 2, Command: "b"},
			{Index: 3, Term: 2, Command: "c"},
			{Index: 4, Term: 2, Command: "d"},
		})
	})

	t.Run("removes conflicting entries", func(t *testing.T) {
		node := createNodeMock(3, 1)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 3, Command: "b"},
			{Index: 3, Term: 3, Command: "c"},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 1, PrevLogTerm: 1, Term: 4, Entries: []raft_state.LogEntry{
			{Index: 3, Term: 4, Command: "e"},
			{Index: 4, Term: 4, Command: "f"},
		}}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 4)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 3, Term: 4, Command: "e"},
			{Index: 4, Term: 4, Command: "f"},
		})
	})

	t.Run("updates commit index to leader commit index when leader's committed entry in log", func(t *testing.T) {
		node := createNodeMock(1, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 1, Command: "b"},
		}

		leaderCommitIndex := uint(3)
		command := raft_commands.AppendEntriesCommand{
			PrevLogIndex:      2,
			PrevLogTerm:       1,
			Term:              2,
			LeaderCommitIndex: leaderCommitIndex,
			Entries: []raft_state.LogEntry{
				{Index: 3, Term: 2, Command: "c"},
				{Index: 4, Term: 2, Command: "d"},
			},
		}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 2)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 1, Command: "b"},
			{Index: 3, Term: 2, Command: "c"},
			{Index: 4, Term: 2, Command: "d"},
		})
		if node.VolatileState.CommitIndex != leaderCommitIndex {
			t.Fatalf("expected commit index to equal %d, got %d", leaderCommitIndex, node.VolatileState.CommitIndex)
		}
	})

	t.Run("updates commit index to last entry index when leader's committed entry not in log", func(t *testing.T) {
		node := createNodeMock(1, 2)
		node.PersistentState.Log = []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 1, Command: "b"},
		}

		lastEntryIndex := uint(4)
		leaderCommitIndex := uint(5)
		command := raft_commands.AppendEntriesCommand{
			PrevLogIndex:      2,
			PrevLogTerm:       1,
			Term:              2,
			LeaderCommitIndex: leaderCommitIndex,
			Entries: []raft_state.LogEntry{
				{Index: 3, Term: 2, Command: "c"},
				{Index: lastEntryIndex, Term: 2, Command: "d"},
			},
		}

		result := handleRaftCommand(node, &command)

		assertResult(t, result, true, 2)
		assertLogEntries(t, node.PersistentState.Log, []raft_state.LogEntry{
			{Index: 1, Term: 1, Command: "a"},
			{Index: 2, Term: 1, Command: "b"},
			{Index: 3, Term: 2, Command: "c"},
			{Index: lastEntryIndex, Term: 2, Command: "d"},
		})
		if node.VolatileState.CommitIndex != lastEntryIndex {
			t.Fatalf("expected commit index to equal %d, got %d", lastEntryIndex, node.VolatileState.CommitIndex)
		}
	})
}
