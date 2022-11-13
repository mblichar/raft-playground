package node

import (
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_state"
	"testing"
)

type stateMock struct {
	persistentState raft_state.PersistentState
	volatileState   raft_state.VolatileState
}

func (state *stateMock) PersistentState() *raft_state.PersistentState {
	return &state.persistentState
}

func (state *stateMock) VolatileState() *raft_state.VolatileState {
	return &state.volatileState
}

func TestHandleAppendEntries(t *testing.T) {
	commandHandler := followerCommandHandler{}
	const stateTerm = uint(2)

	createStateMock := func() stateMock {
		return stateMock{
			persistentState: raft_state.PersistentState{
				CurrentTerm: stateTerm,
			},
		}
	}

	expectSuccessFalse := func(t *testing.T, result raft_commands.AppendEntriesResult) {
		if result.Success {
			t.Fatal("expected success to be false, got true")
		}

		if result.Term != stateTerm {
			t.Fatalf("expected result term to be %d, got %d", stateTerm, result.Term)
		}
	}

	t.Run("returns success: false when command term < state current term", func(t *testing.T) {
		state := createStateMock()
		state.persistentState.CurrentTerm = stateTerm
		command := raft_commands.AppendEntriesCommand{Term: 1}

		result := commandHandler.handleAppendEntries(&state, command)

		expectSuccessFalse(t, result)
	})

	t.Run("returns success: false when no log entry matching command prev log index", func(t *testing.T) {
		state := createStateMock()
		state.persistentState.Log = []raft_state.LogEntry{
			{Index: 1},
			{Index: 2},
		}
		command := raft_commands.AppendEntriesCommand{PrevLogIndex: 3}

		result := commandHandler.handleAppendEntries(&state, command)

		expectSuccessFalse(t, result)
	})
}
