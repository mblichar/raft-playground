package config

type config struct {
	// Election timeout in milliseconds
	ElectionTimeout uint
	// Leader heartbeat timeout in milliseconds
	HeartbeatTimeout uint
	// Network call retry timeout in milliseconds
	RetryTimeout uint
	// Array of raft nodes ids
	RaftNodesIds []uint
}

var Config = config{}
