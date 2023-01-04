package node

import (
	"github.com/go-test/deep"
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_state"
	"testing"
	"time"
)

func createNetworkingMockWithResponses(responses nodeResponseMocks) *raftNetworkingMock {
	return &raftNetworkingMock{requestVoteResponses: responses}
}

func createNetworkingMockWithResponse(response responseMock) raftNetworkingMock {
	return raftNetworkingMock{requestVoteDefaultResponse: &response}
}

func createFailingNetworkingMock() *raftNetworkingMock {
	return &raftNetworkingMock{requestVoteDefaultResponse: &responseMock{err: true}}
}

func createSuccessNetworkingMock() *raftNetworkingMock {
	return &raftNetworkingMock{requestVoteDefaultResponse: &responseMock{
		err:    false,
		result: raft_commands.RaftCommandResult{Success: true},
	}}
}

func createNode() *Node {
	return &Node{
		PersistentState: raft_state.PersistentState{
			CurrentTerm: 3,
			NodeId:      2,
			Log:         []raft_state.LogEntry{{Index: 2, Term: 2, Command: "a"}},
		},
		VolatileState: raft_state.VolatileState{
			Role: raft_state.Follower,
		},
		electionResultChannel:    make(chan electionResult),
		electionCancelledChannel: make(chan struct{}),
	}
}

func assertChannelClosed(t *testing.T, channel chan struct{}) {
	select {
	case <-time.After(time.Second):
		t.Fatalf("expected channel to be closed but it isn't")
	case <-channel:
		return
	}
}

func assertSentCommands(t *testing.T, sentCommands sentRequestVoteCommands, expectedCommands sentRequestVoteCommands) {
	if diff := deep.Equal(sentCommands, expectedCommands); diff != nil {
		t.Fatalf("expected sent commands to match, got the follwoing differences %s", diff)
	}
}

func assertRetryTimeout(t *testing.T, timeout *timeoutMock) {
	if timeout.kind != "request-vote-retry" {
		t.Fatalf("expected timeout kind to equal request-vote-retry, got %s", timeout.kind)
	}

	if timeout.milliseconds != config.Config.RetryTimeout {
		t.Fatalf("expected timeout milliseconds to equal request-vote-retry, got %d", timeout.milliseconds)
	}
}

func TestLeaderElection(t *testing.T) {
	config.Config.RaftNodesIds = []uint{1, 2, 3}
	config.Config.RetryTimeout = 1337

	t.Run("cancels previous election", func(t *testing.T) {
		node := createNode()
		prevElectionResultChannel := node.electionResultChannel
		prevElectionCancelledChannel := node.electionCancelledChannel

		startNewElection(node, createFailingNetworkingMock(), &timeoutFactoryMock{})

		if prevElectionResultChannel == node.electionResultChannel {
			t.Fatalf("expected electionResultChanel to change")
		}

		if prevElectionCancelledChannel == node.electionCancelledChannel {
			t.Fatalf("expected electionCancelledChanel to change")
		}

		assertChannelClosed(t, prevElectionCancelledChannel)
		close(node.electionCancelledChannel)
	})

	t.Run("proceeds normally when there was no previous election", func(t *testing.T) {
		node := createNode()
		node.electionResultChannel = nil
		node.electionCancelledChannel = nil

		startNewElection(node, createFailingNetworkingMock(), &timeoutFactoryMock{})

		if node.electionResultChannel == nil {
			t.Fatalf("expected electionResultChanel to be set")
		}

		if node.electionCancelledChannel == nil {
			t.Fatalf("expected electionCancelledChanel to be set")
		}

		close(node.electionCancelledChannel)
	})

	t.Run("sends correct RequestVote commands", func(t *testing.T) {
		node := createNode()
		networkingMock := createFailingNetworkingMock()

		lastLogEntry := node.PersistentState.Log[len(node.PersistentState.Log)-1]
		expectedCommand := raft_commands.RequestVoteCommand{
			Term:         node.PersistentState.CurrentTerm + 1,
			CandidateId:  node.PersistentState.NodeId,
			LastLogIndex: lastLogEntry.Index,
			LastLogTerm:  lastLogEntry.Term,
		}
		expectedNodeCommands := sentRequestVoteCommands{
			1: {expectedCommand},
			3: {expectedCommand},
		}

		factoryMock := timeoutFactoryMock{}
		factoryMock.timeoutCreated = make(chan timeoutMock)

		startNewElection(node, networkingMock, &factoryMock)

		// all network calls are failing, wait for retry timeouts which created after command is sent to each node
		for i := 0; i < len(config.Config.RaftNodesIds)-1; i++ {
			<-factoryMock.timeoutCreated
		}

		assertSentCommands(t, networkingMock.sentRequestVoteCommands, expectedNodeCommands)
	})

	t.Run("retries failed calls after correct timeout", func(t *testing.T) {
		node := createNode()

		lastLogEntry := node.PersistentState.Log[len(node.PersistentState.Log)-1]
		expectedCommand := raft_commands.RequestVoteCommand{
			Term:         node.PersistentState.CurrentTerm + 1,
			CandidateId:  node.PersistentState.NodeId,
			LastLogIndex: lastLogEntry.Index,
			LastLogTerm:  lastLogEntry.Term,
		}

		networkingMock := createNetworkingMockWithResponses(nodeResponseMocks{
			1: []responseMock{
				{err: true, result: raft_commands.RaftCommandResult{}},
				{err: true, result: raft_commands.RaftCommandResult{}},
				{err: false, result: raft_commands.RaftCommandResult{Success: false}},
			},
			3: []responseMock{
				{err: false, result: raft_commands.RaftCommandResult{Success: false}},
			},
		})
		networkingMock.sentRequestVoteCommand = make(chan raft_commands.RequestVoteCommand)

		factoryMock := timeoutFactoryMock{}
		factoryMock.timeoutCreated = make(chan timeoutMock)

		startNewElection(node, networkingMock, &factoryMock)

		timeout := <-factoryMock.timeoutCreated
		assertRetryTimeout(t, &timeout)

		// two commands should be sent
		<-networkingMock.sentRequestVoteCommand
		<-networkingMock.sentRequestVoteCommand

		assertSentCommands(t, networkingMock.sentRequestVoteCommands, sentRequestVoteCommands{
			1: {expectedCommand},
			3: {expectedCommand},
		})

		// retry failed command
		<-timeout.doneCalled
		close(timeout.done)

		// another command should be sent
		<-networkingMock.sentRequestVoteCommand
		assertSentCommands(t, networkingMock.sentRequestVoteCommands, sentRequestVoteCommands{
			1: {expectedCommand, expectedCommand},
			3: {expectedCommand},
		})

		// retry once again
		timeout = <-factoryMock.timeoutCreated
		assertRetryTimeout(t, &timeout)
		<-timeout.doneCalled
		close(timeout.done)

		// another command should be sent
		<-networkingMock.sentRequestVoteCommand
		assertSentCommands(t, networkingMock.sentRequestVoteCommands, sentRequestVoteCommands{
			1: {expectedCommand, expectedCommand, expectedCommand},
			3: {expectedCommand},
		})
	})

	t.Run("wins election after receiving enough votes", func(t *testing.T) {
		node := createNode()
		networkingMock := createNetworkingMockWithResponses(nodeResponseMocks{
			1: []responseMock{
				{err: false, result: raft_commands.RaftCommandResult{Success: true}},
			},
			3: []responseMock{
				{err: false, result: raft_commands.RaftCommandResult{Success: false}},
			},
		})

		startNewElection(node, networkingMock, &timeoutFactoryMock{})

		electionResult := <-node.electionResultChannel
		if !electionResult.won {
			t.Fatal("expected electionResult.won to be true, got false")
		}
	})

	t.Run("loses election when there's node with higher term", func(t *testing.T) {
		node := createNode()
		higherTerm := node.PersistentState.CurrentTerm + 2
		networkingMock := createNetworkingMockWithResponses(nodeResponseMocks{
			1: []responseMock{
				{err: false, result: raft_commands.RaftCommandResult{Success: false, Term: higherTerm}},
			},
			3: []responseMock{
				{err: false, result: raft_commands.RaftCommandResult{Success: false}},
			},
		})

		startNewElection(node, networkingMock, &timeoutFactoryMock{})

		electionResult := <-node.electionResultChannel
		if electionResult.won {
			t.Fatal("expected electionResult.won to be false, got true")
		}

		if electionResult.currentTerm != higherTerm {
			t.Fatalf("expected electionResult.current term to be %d, got %d", higherTerm, electionResult.currentTerm)
		}
	})
}
