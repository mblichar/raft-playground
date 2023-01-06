package node

import (
	"fmt"
	"github.com/mblichar/raft/src/raft_commands"
	"github.com/mblichar/raft/src/raft_networking"
	"github.com/mblichar/raft/src/timer"
	"sync"
)

type responseMock struct {
	result raft_commands.RaftCommandResult
	err    bool
}

type sentRequestVoteCommands = map[int][]raft_commands.RequestVoteCommand
type sentAppendEntriesCommands = map[int][]raft_commands.AppendEntriesCommand
type nodeResponseMocks = map[int][]responseMock

type raftNetworkingMock struct {
	mutex                        sync.Mutex
	requestVoteResponses         nodeResponseMocks
	appendEntriesResponses       nodeResponseMocks
	requestVoteDefaultResponse   *responseMock
	appendEntriesDefaultResponse *responseMock
	sentRequestVoteCommands      sentRequestVoteCommands
	sentRequestVoteCommand       chan raft_commands.RequestVoteCommand
	sentAppendEntriesCommands    sentAppendEntriesCommands
	sentAppendEntriesCommand     chan raft_commands.AppendEntriesCommand
}

func (_ *raftNetworkingMock) ListenForRaftCommands() chan raft_networking.CommandWrapper {
	return nil
}

func (mock *raftNetworkingMock) SendAppendEntriesCommand(
	nodeId uint,
	command raft_commands.AppendEntriesCommand,
) (raft_commands.RaftCommandResult, bool) {
	mock.mutex.Lock()
	response := getResponseMock(nodeId, mock.appendEntriesResponses, mock.appendEntriesDefaultResponse)

	if mock.sentAppendEntriesCommands == nil {
		mock.sentAppendEntriesCommands = make(map[int][]raft_commands.AppendEntriesCommand)
	}

	if commands, exists := mock.sentAppendEntriesCommands[int(nodeId)]; exists {
		mock.sentAppendEntriesCommands[int(nodeId)] = append(commands, command)
	} else {
		mock.sentAppendEntriesCommands[int(nodeId)] = []raft_commands.AppendEntriesCommand{command}
	}

	if mock.sentAppendEntriesCommand != nil {
		go func() {
			mock.sentAppendEntriesCommand <- command
		}()
	}

	mock.mutex.Unlock()
	return response.result, response.err
}
func (mock *raftNetworkingMock) SendRequestVoteCommand(
	nodeId uint,
	command raft_commands.RequestVoteCommand,
) (raft_commands.RaftCommandResult, bool) {
	mock.mutex.Lock()
	response := getResponseMock(nodeId, mock.requestVoteResponses, mock.requestVoteDefaultResponse)

	if mock.sentRequestVoteCommands == nil {
		mock.sentRequestVoteCommands = make(map[int][]raft_commands.RequestVoteCommand)
	}

	if commands, exists := mock.sentRequestVoteCommands[int(nodeId)]; exists {
		mock.sentRequestVoteCommands[int(nodeId)] = append(commands, command)
	} else {
		mock.sentRequestVoteCommands[int(nodeId)] = []raft_commands.RequestVoteCommand{command}
	}

	if mock.sentRequestVoteCommand != nil {
		go func() {
			mock.sentRequestVoteCommand <- command
		}()
	}

	mock.mutex.Unlock()
	return response.result, response.err
}

func getResponseMock(nodeId uint, responsesMap map[int][]responseMock, defaultResponse *responseMock) responseMock {
	responses, exists := responsesMap[int(nodeId)]
	if exists {
		if len(responses) == 0 {
			panic(any(fmt.Sprintf("node %d called more times than expected - no more response mocks", nodeId)))
		}

		response := responses[0]
		responsesMap[int(nodeId)] = responses[1:]

		return response
	}

	if defaultResponse != nil {
		return *defaultResponse
	}

	panic(any(fmt.Sprintf("node %d called but no response mock nor default response provided", nodeId)))
}

type timeoutFactoryMock struct {
	timeouts       []timeoutMock
	timeoutCreated chan timeoutMock
}

type timeoutMock struct {
	kind         string
	milliseconds uint
	done         chan struct{}
	doneCalled   chan struct{}
}

func (mock *timeoutFactoryMock) Timeout(kind string, milliseconds uint) timer.Timeout {
	timeout := timeoutMock{
		kind:         kind,
		milliseconds: milliseconds,
		done:         make(chan struct{}),
		doneCalled:   make(chan struct{}),
	}

	if mock.timeouts == nil {
		mock.timeouts = []timeoutMock{}
	}

	mock.timeouts = append(mock.timeouts, timeout)

	if mock.timeoutCreated != nil {
		go func() {
			// TODO: fix that to ensure timeouts added in order
			mock.timeoutCreated <- timeout
		}()
	}

	return &timeout
}

func (mock *timeoutMock) Done() <-chan struct{} {
	go func() {
		close(mock.doneCalled)
	}()
	return mock.done
}
