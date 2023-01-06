package cli

import (
	"fmt"
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/logging"
	"github.com/mblichar/raft/src/node"
	"github.com/mblichar/raft/src/raft_state"
	"github.com/rivo/tview"
	"strings"
	"time"
)

func StartCli() {
	// create and start nodes
	nodes := make([]*node.Node, len(config.Config.NodeIds))
	nodesById := make(map[uint]*node.Node)
	nodeQuitChannels := make(map[uint]chan struct{})
	nodeTimeoutFactories := make(map[uint]*controllableTimeoutFactory)
	logs := make(chan logging.LoggerEntry, 1000)
	networkingController := createNetworkController(
		logging.CreateLogger("[NETWORK]", logs),
	)

	for idx, nodeId := range config.Config.NodeIds {
		quit := make(chan struct{})
		timeoutFactory := createControllableTimeoutFactory(nodeId)

		n := node.CreateNode(nodeId)
		nodes[idx] = n
		nodesById[nodeId] = n
		nodeQuitChannels[nodeId] = quit
		nodeTimeoutFactories[nodeId] = timeoutFactory

		networking := controllableNetworking{
			controller: networkingController,
			nodeId:     nodeId,
		}

		logger := logging.CreateLogger(fmt.Sprintf("[NODE %d]", nodeId), logs)

		go node.StartProcessingLoop(n, &networking, &networking, timeoutFactory, logger, quit)
	}

	app, appQuit := setupApp(nodes, logs)

	if err := app.Run(); err != nil {
		panic(any(err))
	}

	close(appQuit)
}

func setupApp(nodes []*node.Node, logs chan logging.LoggerEntry) (*tview.Application, chan struct{}) {
	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)

	nodesStateTextView := tview.NewTextView()
	nodesStateTextView.SetBorder(true).SetTitle("Nodes State")
	flex.AddItem(nodesStateTextView, 0, 2, false)

	loggerTextView := tview.NewTextView()
	loggerTextView.SetBorder(true).SetTitle("Logs")
	flex.AddItem(loggerTextView, 0, 3, false)

	commandsInputField := tview.NewInputField()
	commandsInputField.SetBorder(true).SetTitle("Commands Input")
	flex.AddItem(commandsInputField, 3, 1, true)

	appQuit := make(chan struct{})

	app := tview.NewApplication().SetRoot(flex, true)

	go renderLogs(logs, loggerTextView, appQuit)
	go func() {
		for {
			select {
			case <-time.After(100 * time.Millisecond):
				renderNodesState(nodes, nodesStateTextView)
				app.Draw()
			case <-appQuit:
				return
			}
		}
	}()
	return app, appQuit
}

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
		fmt.Fprintf(writer, "LOG: %s", logEntriesToString(n.PersistentState.Log))
		fmt.Fprintf(writer, "\n")
		fmt.Fprintf(writer, "\n")
	}
}

func renderLogs(logs chan logging.LoggerEntry, textView *tview.TextView, quit chan struct{}) {
	start := time.Now()
	for {
		select {
		case entry := <-logs:
			writer := textView.BatchWriter()
			prefix := formatTimestamp(start, entry.Timestamp)
			for _, message := range entry.Messages {
				fmt.Fprintf(writer, "%s %s\n", prefix, message)
				prefix = strings.Repeat(" ", len(prefix))
			}
			writer.Close()
		case <-quit:
			return
		}
	}
}

func formatTimestamp(start time.Time, end time.Time) string {
	diff := end.Sub(start)
	return fmt.Sprintf("[%02d:%02d:%04d]", int(diff.Minutes()), int(diff.Seconds()), diff.Milliseconds())
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
