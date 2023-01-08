package cli

import (
	"fmt"
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/logging"
	"github.com/mblichar/raft/src/node"
	"github.com/mblichar/raft/src/raft_state"
	"github.com/rivo/tview"
	"sort"
	"strings"
	"time"
)

func renderNodesState(nodes []*node.Node, textView *tview.TextView) {
	writer := textView.BatchWriter()
	writer.Clear()
	defer writer.Close()

	for _, n := range nodes {
		fmt.Fprintf(writer, "NODE: %d  ROLE: %10s  TERM: %2d  VOTED: %2d  COMMIT: %2d  APPLIED: %2d  LEADER: %d\n",
			n.PersistentState.NodeId,
			roleToString(n.VolatileState.Role),
			n.PersistentState.CurrentTerm,
			n.PersistentState.VotedFor,
			n.VolatileState.CommitIndex,
			n.VolatileState.LastApplied,
			n.VolatileState.LeaderId,
		)
		fmt.Fprintf(writer, "LOG: %s\n", logEntriesToString(n.PersistentState.Log))
		fmt.Fprintf(writer, "APP DB: %s\n", applicationDbToString(n))
		fmt.Fprintf(writer, "\n")
	}
}

func applicationDbToString(n *node.Node) string {
	keys := make([]string, len(n.ApplicationDatabase))
	for key, _ := range n.ApplicationDatabase {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := "{ "
	for _, key := range keys {
		// ApplicationDatabase is being concurrently updated so the key may no longer be here,
		// it can be ignored silently, next re-render should fix the displayed state
		if value, ok := n.ApplicationDatabase[key]; ok {
			result += fmt.Sprintf("%s: %s ", key, value)
		}
	}
	result += "}"

	return result
}

func renderLogs(logs chan logging.LoggerEntry, textView *tview.TextView, quit chan struct{}) {
	start := time.Now()
	for {
		select {
		case entry := <-logs:
			writer := textView.BatchWriter()
			prefix := formatTimestamp(start, entry.Timestamp)
			for _, message := range entry.Messages {
				fmt.Fprintf(writer, "[white]%s %s\n", prefix, message)
				prefix = strings.Repeat(" ", len(prefix))
			}
			writer.Close()
		case <-quit:
			return
		}
	}
}

func renderConfig(context *appContext, textView *tview.TextView) {
	writer := textView.BatchWriter()
	writer.Clear()
	splitsString := ""
	for _, split := range context.networkController.networkSplits {
		splitsString += fmt.Sprintf("%d", split[0])

		for _, nodeId := range split[1:] {
			splitsString += fmt.Sprintf(",%d", nodeId)
		}

		splitsString += " "
	}

	fmt.Fprintf(writer,
		"ELECTION TIMEOUT: %dms  HEARTBEAT TIMEOUT: %dms  RETRY TIMEOUT: %dms  NETWORK LATENCY: %dms  NETWORK SPLITS: %s",
		config.Config.ElectionTimeout, config.Config.HeartbeatTimeout, config.Config.RetryTimeout,
		config.Config.NetworkLatency, splitsString)
	writer.Close()

}

func formatTimestamp(start time.Time, end time.Time) string {
	diff := end.Sub(start)
	return fmt.Sprintf("[%02d:%02d:%04d]", int(diff.Minutes()), int(diff.Seconds())%60, diff.Milliseconds()%1000)
}

func roleToString(role raft_state.NodeRole) string {
	switch role {
	case raft_state.Leader:
		return "LEADER"
	case raft_state.Follower:
		return "FOLLOWER"
	case raft_state.Candidate:
		return "CANDIDATE"
	default:
		return "UNKNOWN"
	}
}

func logEntriesToString(entries []raft_state.LogEntry) string {
	result := ""
	for _, entry := range entries {
		result += fmt.Sprintf("[I:%d T:%d C:'%s']", entry.Index, entry.Term, entry.Command)
	}

	return result
}
