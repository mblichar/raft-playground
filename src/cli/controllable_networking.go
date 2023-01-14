package cli

import (
	"fmt"
	"github.com/mblichar/raft-playground/src/client_networking"
	"github.com/mblichar/raft-playground/src/config"
	"github.com/mblichar/raft-playground/src/logging"
	"github.com/mblichar/raft-playground/src/raft_commands"
	"github.com/mblichar/raft-playground/src/raft_networking"
	"math/rand"
	"time"
)

type networkController struct {
	networkSplits        [][]uint
	raftListenChannels   map[uint]chan raft_networking.CommandWrapper
	clientListenChannels map[uint]chan client_networking.CommandWrapper
	logger               *logging.Logger
}

type controllableNetworking struct {
	nodeId     uint
	controller *networkController
}

func createNetworkController(logger *logging.Logger) *networkController {
	controller := networkController{
		raftListenChannels:   make(map[uint]chan raft_networking.CommandWrapper),
		clientListenChannels: make(map[uint]chan client_networking.CommandWrapper),
		logger:               logger,
	}

	for _, nodeId := range config.Config.NodeIds {
		controller.raftListenChannels[nodeId] = make(chan raft_networking.CommandWrapper, 1000)
		controller.clientListenChannels[nodeId] = make(chan client_networking.CommandWrapper, 1000)
	}

	controller.networkSplits = make([][]uint, 1)
	controller.networkSplits[0] = make([]uint, len(config.Config.NodeIds))
	copy(controller.networkSplits[0], config.Config.NodeIds)

	return &controller
}

func (networking *controllableNetworking) ListenForRaftCommands() chan raft_networking.CommandWrapper {
	return networking.controller.raftListenChannels[networking.nodeId]
}

func (networking *controllableNetworking) SendAppendEntriesCommand(
	nodeId uint,
	command raft_commands.AppendEntriesCommand,
) (raft_commands.RaftCommandResult, bool) {
	return networking.sendRaftCommand(nodeId, &command)
}

func (networking *controllableNetworking) SendRequestVoteCommand(
	nodeId uint,
	command raft_commands.RequestVoteCommand,
) (raft_commands.RaftCommandResult, bool) {
	return networking.sendRaftCommand(nodeId, &command)
}

func (networking *controllableNetworking) ListenForClientCommands() chan client_networking.CommandWrapper {
	return networking.controller.clientListenChannels[networking.nodeId]
}

func (networking *controllableNetworking) SendCommand(
	nodeId uint,
	command string,
) (client_networking.CommandResult, bool) {
	if networking.controller.canConnect(networking.nodeId, nodeId) {
		latency := time.Duration(config.Config.NetworkLatency) * time.Millisecond
		<-time.After(latency + time.Duration(rand.Intn(int(latency)/4)))

		result := make(chan client_networking.CommandResult)
		networking.controller.clientListenChannels[nodeId] <- client_networking.CommandWrapper{
			Command: command,
			Result:  result,
		}

		return <-result, false
	} else {
		return client_networking.CommandResult{}, true
	}
}

func (networking *controllableNetworking) sendRaftCommand(
	nodeId uint,
	command raft_commands.RaftCommand,
) (raft_commands.RaftCommandResult, bool) {
	if networking.controller.canConnect(networking.nodeId, nodeId) {
		latency := time.Duration(config.Config.NetworkLatency) * time.Millisecond
		<-time.After(latency + time.Duration(rand.Intn(int(latency)/4)))

		result := make(chan raft_commands.RaftCommandResult)
		networking.controller.raftListenChannels[nodeId] <- raft_networking.CommandWrapper{
			Command: command,
			Result:  result,
		}
		logRaftCommand(networking.controller.logger, networking.nodeId, nodeId, command, true)

		r := <-result
		networking.controller.logger.Log(fmt.Sprintf("%s Result(Term: %d, Success: %t)",
			logPrefix(nodeId, networking.nodeId), r.Term, r.Success))
		return r, false
	} else {
		logRaftCommand(networking.controller.logger, networking.nodeId, nodeId, command, false)
		return raft_commands.RaftCommandResult{}, true
	}
}

func logRaftCommand(logger *logging.Logger, senderId uint, receiverId uint, command raft_commands.RaftCommand, success bool) {
	prefix := logPrefix(senderId, receiverId)
	switch command.CommandType() {
	case raft_commands.AppendEntries:
		c := command.ToAppendEntries()
		if success {
			logger.Log(
				fmt.Sprintf("%s AppendEntries(Term: %d LeaderId: %d PrevLogIndex: %d PrevLogTerm: %d Entries: %s Commit: %d)",
					prefix, c.Term, c.LeaderId, c.PrevLogIndex, c.PrevLogTerm, logEntriesToString(c.Entries), c.LeaderCommitIndex),
			)
		} else {
			logger.Log(fmt.Sprintf("%s failed to send AppendEntries - node unreachable", prefix))
		}
	case raft_commands.RequestVote:
		c := command.ToRequestVote()

		if success {
			logger.Log(
				fmt.Sprintf("%s RequestVote(Term: %d CandidateId: %d LastLogIndex: %d LastLogTerm: %d)",
					prefix, c.Term, c.CandidateId, c.LastLogIndex, c.LastLogTerm),
			)
		} else {
			logger.Log(fmt.Sprintf("%s failed to send RequestVote - node unreachable", prefix))
		}
	}
}

func logPrefix(senderId uint, receiverId uint) string {
	prefix := fmt.Sprintf("%d->%d", senderId, receiverId)
	return prefix
}

func (controller *networkController) canConnect(a uint, b uint) bool {
	for _, split := range controller.networkSplits {
		if sliceContains(split, a) && sliceContains(split, b) {
			return true
		}
	}

	return false
}

func sliceContains(s []uint, x uint) bool {
	for _, val := range s {
		if val == x {
			return true
		}
	}

	return false
}
