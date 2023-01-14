package config

type config struct {
	// Election timeout in milliseconds
	ElectionTimeout int
	// Leader heartbeat timeout in milliseconds
	HeartbeatTimeout int
	// Network call retry timeout in milliseconds
	RetryTimeout int
	// Network latency in milliseconds
	NetworkLatency int
	// Array of raft-playground nodes ids
	NodeIds []uint
}

var Config = config{}
