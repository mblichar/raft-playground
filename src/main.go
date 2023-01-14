package main

import (
	"github.com/mblichar/raft-playground/src/cli"
	"github.com/mblichar/raft-playground/src/config"
)

func main() {
	config.Config.NodeIds = []uint{1, 2, 3, 4, 5}
	config.Config.RetryTimeout = 3000
	config.Config.ElectionTimeout = 10000
	config.Config.HeartbeatTimeout = 5000
	config.Config.NetworkLatency = 1000

	cli.StartCli()
}
