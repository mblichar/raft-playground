package cli

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/mblichar/raft/src/config"
	"github.com/mblichar/raft/src/logging"
	"github.com/mblichar/raft/src/node"
	"github.com/rivo/tview"
	"time"
)

type appContext struct {
	nodes                []*node.Node
	nodesById            map[uint]*node.Node
	nodeQuitChannels     map[uint]chan struct{}
	nodeTimeoutFactories map[uint]*controllableTimeoutFactory
	logs                 chan logging.LoggerEntry
	networkController    *networkController
}

func StartCli() {
	context := appContext{
		nodes:                make([]*node.Node, len(config.Config.NodeIds)),
		nodesById:            make(map[uint]*node.Node),
		nodeQuitChannels:     make(map[uint]chan struct{}),
		nodeTimeoutFactories: make(map[uint]*controllableTimeoutFactory),
		logs:                 make(chan logging.LoggerEntry, 1000),
	}
	context.networkController = createNetworkController(
		logging.CreateLogger("[yellow][NETWORK[]", context.logs),
	)

	for idx, nodeId := range config.Config.NodeIds {
		n := node.CreateNode(nodeId)
		context.nodes[idx] = n
		context.nodesById[nodeId] = n

		startNode(nodeId, n, &context)
	}

	app, appQuit := setupApp(&context)

	if err := app.Run(); err != nil {
		panic(any(err))
	}

	close(appQuit)
}

func setupApp(context *appContext) (*tview.Application, chan struct{}) {
	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)

	nodesStateTextView := tview.NewTextView()
	nodesStateTextView.SetBorder(true).SetTitle("Nodes State")
	flex.AddItem(nodesStateTextView, 0, 2, false)

	loggerTextView := tview.NewTextView()
	loggerTextView.SetBorder(true).SetTitle("Logs")
	loggerTextView.SetDynamicColors(true)
	flex.AddItem(loggerTextView, 0, 3, false)

	configTextView := tview.NewTextView()
	configTextView.SetBorder(true).SetTitle("Config")
	flex.AddItem(configTextView, 3, 1, false)

	commandsInputField := tview.NewInputField()
	commandsInputField.SetBorder(true).SetTitle("Commands Input")
	commandsInputField.SetPlaceholder(
		"(enter commands, type 'help' to get more information about commands, use tab to switch between logs and commands, use j/k to scroll through logs)")
	commandsInputField.SetPlaceholderTextColor(tcell.ColorGray)
	flex.AddItem(commandsInputField, 3, 1, true)

	appQuit := make(chan struct{})

	app := tview.NewApplication().SetRoot(flex, true)
	app.SetFocus(commandsInputField)

	commandInputFocused := true
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			if commandInputFocused {
				app.SetFocus(loggerTextView)
			} else {
				app.SetFocus(commandsInputField)
			}
			commandInputFocused = !commandInputFocused
		}
		return event
	})

	logging.CreateLogger("[INFO[]", context.logs).Log("Wait for initial election to elect a leader...")

	go renderLogs(context.logs, loggerTextView, appQuit)
	go listenForUserCommands(commandsInputField, context, appQuit)
	go func() {
		for {
			select {
			case <-time.After(100 * time.Millisecond):
				renderNodesState(context.nodes, nodesStateTextView)
				renderConfig(context, configTextView)
				app.Draw()
			case <-appQuit:
				return
			}
		}
	}()
	return app, appQuit
}

func startNode(nodeId uint, n *node.Node, context *appContext) {
	quit := make(chan struct{})
	timeoutFactory := createControllableTimeoutFactory(nodeId)

	context.nodeQuitChannels[nodeId] = quit
	context.nodeTimeoutFactories[nodeId] = timeoutFactory

	networking := controllableNetworking{
		controller: context.networkController,
		nodeId:     nodeId,
	}

	logger := logging.CreateLogger(fmt.Sprintf("[gray][NODE %d]", nodeId), context.logs)

	go node.StartProcessingLoop(n, &networking, &networking, timeoutFactory, logger, quit)
}
