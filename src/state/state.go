package state

// NilVotedFor Constant indicating that given node has not voted yet
const NilVotedFor = -1

type LogEntry struct {
	// Command for a state machine
	Command []byte
	// Term in which entry was received by leader
	Term uint
}

// PersistentState struct for persistent state kept on all nodes
type PersistentState struct {
	// Latest term server has seen
	CurrentTerm uint
	// Id of candidate that received vote in current term (NilVotedFor if none)
	VotedFor int
	// Log entries for state machine
	Log []LogEntry
}

// VolatileNodeState struct for volatile state kept on all nodes
type VolatileNodeState struct {
	// Index of highest log entry known to be committed
	CommitIndex uint
	// Index of highest log entry applied to state machine
	LastApplied uint
}

// VolatileLeaderState struct for volatile state kept only on leader (reinitialized after election)
type VolatileLeaderState struct {
	// Index of the next log entry to send for given node (array index is equal to node id)
	NextIndex []uint
	// Index of highest log entry known to be replicated on given node (array index is equal to node id)
	MatchIndex []uint
}

// State struct for keeping all node state
type State struct {
	PersistentState
	VolatileNodeState
	*VolatileLeaderState
}
