package cli

import (
	"github.com/mblichar/raft-playground/src/timer"
	"sync"
	"time"
)

type controllableTimeout struct {
	kind         string
	milliseconds int
	done         chan struct{}
}

type regularTimeout struct {
	timeout <-chan time.Time
}

func (timeout *controllableTimeout) Done() <-chan struct{} {
	return timeout.done
}

func (timeout *regularTimeout) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-timeout.timeout
		close(done)
	}()
	return done
}

type controllableTimeoutFactory struct {
	mutex                sync.Mutex
	currentId            int
	nodeId               uint
	controlTimeouts      bool
	controllableTimeouts map[int]*controllableTimeout
}

func createControllableTimeoutFactory(nodeId uint) *controllableTimeoutFactory {
	return &controllableTimeoutFactory{
		nodeId:               nodeId,
		controllableTimeouts: make(map[int]*controllableTimeout),
	}
}

func (factory *controllableTimeoutFactory) Timeout(kind string, milliseconds int) timer.Timeout {
	if factory.controlTimeouts {
		factory.mutex.Lock()
		factory.currentId++
		timeout := controllableTimeout{kind: kind, milliseconds: milliseconds}
		factory.controllableTimeouts[factory.currentId] = &timeout
		factory.mutex.Unlock()
		return &timeout
	} else {
		return &regularTimeout{timeout: time.After(time.Duration(milliseconds) * time.Millisecond)}
	}
}
