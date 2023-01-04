package client_networking

type CommandWrapper struct {
	Command string
	Result  chan<- CommandResult
}

type CommandResult struct {
	Result  string
	Success bool
}

type ClientNetworking interface {
	ListenForCommands() chan CommandWrapper
	SendCommand(nodeId uint, command string) (CommandResult, bool)
}
