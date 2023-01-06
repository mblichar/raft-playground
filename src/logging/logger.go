package logging

import (
	"fmt"
	"time"
)

type LoggerEntry struct {
	Messages  []string
	Timestamp time.Time
}

type Logger struct {
	Logs   chan LoggerEntry
	prefix string
}

func CreateLogger(prefix string, logs chan LoggerEntry) *Logger {
	return &Logger{
		Logs:   logs,
		prefix: prefix,
	}
}

func (logg *Logger) Log(message string) {
	logg.Logs <- LoggerEntry{
		Messages: []string{
			fmt.Sprintf("%s %s", logg.prefix, message),
		},
		Timestamp: time.Now(),
	}
}

func (logg *Logger) LogMultiple(messages []string) {
	for idx, message := range messages {
		messages[idx] = fmt.Sprintf("%s %s", logg.prefix, message)
	}
	logg.Logs <- LoggerEntry{
		Messages:  messages,
		Timestamp: time.Now(),
	}
}
