package pkg

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI manages the chat room interface and user interactions.
type UI struct {
	*ChatRoom
	App        *tview.Application
	MsgInputs  chan string
	CmdInputs  chan UICommand
	PeerBox    *tview.TextView
	MessageBox *tview.TextView
	InputBox   *tview.InputField
}

// UICommand represents a user input command.
type UICommand struct {
	CommandType string
	Argument    string
}

// NewUI initializes the user interface for a given ChatRoom.
func NewUI(cr *ChatRoom) *UI {
	app := tview.NewApplication()

	cmdChan := make(chan UICommand, 1)
	msgChan := make(chan string, 1)

	titleBox := createTitleBox()
	messageBox := createMessageBox(cr.RoomName)
	usageBox := createUsageBox()
	peerBox := createPeerBox()
	inputField := createInputField(cr.UserName, cmdChan, msgChan)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleBox, 3, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(messageBox, 0, 1, false).
			AddItem(peerBox, 20, 1, false), 0, 8, false).
		AddItem(inputField, 3, 1, true).
		AddItem(usageBox, 3, 1, false)

	app.SetRoot(layout, true)

	return &UI{
		ChatRoom:   cr,
		App:        app,
		PeerBox:    peerBox,
		MessageBox: messageBox,
		InputBox:   inputField,
		MsgInputs:  msgChan,
		CmdInputs:  cmdChan,
	}
}

// Run starts the application UI.
func (ui *UI) Run() error {
	go ui.handleEvents()
	return ui.App.Run()
}

// Close stops the UI and leaves the chat room.
func (ui *UI) Close() {
	ui.psCancel()
	ui.App.Stop()
}

// handleEvents processes user inputs, logs, and peer updates.
func (ui *UI) handleEvents() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-ui.MsgInputs:
			ui.Outbound <- msg
			ui.displayMessage(ui.UserName, msg, tcell.ColorGreen)
		case cmd := <-ui.CmdInputs:
			ui.processCommand(cmd)
		case msg := <-ui.Inbound:
			ui.displayMessage(msg.SenderName, msg.Message, tcell.ColorBlue)
		case log := <-ui.Logs:
			ui.displayLog(log)
		case <-ticker.C:
			ui.updatePeerBox()
		case <-ui.psCtx.Done():
			return
		}
	}
}

// processCommand interprets and executes user commands.
func (ui *UI) processCommand(cmd UICommand) {
	switch cmd.CommandType {
	case "/exit":
		ui.Close()
	case "/clear":
		ui.App.QueueUpdateDraw(func() {
			ui.MessageBox.Clear()
		})
	case "/room":
		if cmd.Argument == "" {
			ui.Logs <- chatLog{Prefix: "error", Msg: "missing room name"}
		} else {
			ui.switchRoom(cmd.Argument)
		}
	case "/user":
		if cmd.Argument == "" {
			ui.Logs <- chatLog{Prefix: "error", Msg: "missing username"}
		} else {
			ui.UpdateUser(cmd.Argument)
			ui.InputBox.SetLabel(ui.UserName + " > ")
		}
	default:
		ui.Logs <- chatLog{Prefix: "error", Msg: fmt.Sprintf("unsupported command: %s", cmd.CommandType)}
	}
}

// switchRoom switches the chat room.
func (ui *UI) switchRoom(roomName string) {
	ui.Logs <- chatLog{Prefix: "info", Msg: fmt.Sprintf("switching to room '%s'", roomName)}

	newChatRoom, err := JoinChatRoom(ui.Host, ui.UserName, roomName)
	if err != nil {
		ui.Logs <- chatLog{Prefix: "error", Msg: fmt.Sprintf("could not switch rooms: %s", err)}
		return
	}

	ui.ChatRoom.Exit()
	ui.ChatRoom = newChatRoom
	time.Sleep(time.Second)

	ui.App.QueueUpdateDraw(func() {
		ui.MessageBox.Clear()
		ui.MessageBox.SetTitle(fmt.Sprintf("ChatRoom-%s", ui.ChatRoom.RoomName))
	})
}

// displayMessage renders messages in the message box.
func (ui *UI) displayMessage(sender, message string, color tcell.Color) {
	ui.App.QueueUpdateDraw(func() {
		fmt.Fprintf(ui.MessageBox, "[%s]<%s>[-] %s\n", color, sender, message)
		ui.MessageBox.ScrollToEnd()
	})
}

// displayLog renders logs in the message box.
func (ui *UI) displayLog(log chatLog) {
	ui.App.QueueUpdateDraw(func() {
		fmt.Fprintf(ui.MessageBox, "[red](%s)[-] %s\n", log.Prefix, log.Msg)
		ui.MessageBox.ScrollToEnd()
	})
}

// updatePeerBox refreshes the list of peers.
func (ui *UI) updatePeerBox() {
	ui.App.QueueUpdateDraw(func() {
		ui.PeerBox.Clear()

		for _, peer := range ui.ChatRoom.PeerList() {
			shortID := peer.Pretty()[len(peer.Pretty())-8:]
			fmt.Fprintf(ui.PeerBox, "[yellow]%s[-]\n", shortID)
		}
	})
}

// UI Helper Functions

func createTitleBox() *tview.TextView {
	titleBox := tview.NewTextView().
		SetText("Welcome to PeerNet.").
		SetTextColor(tcell.ColorWhite).
		SetTextAlign(tview.AlignCenter)
	titleBox.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("PeerNet").
		SetTitleColor(tcell.ColorWhite).
		SetTitleAlign(tview.AlignCenter)
	return titleBox
}

func createMessageBox(roomName string) *tview.TextView {
	messageBox := tview.NewTextView().
		SetDynamicColors(true)
	messageBox.SetBorder(true).SetBorderColor(tcell.ColorGreen).
		SetTitle(fmt.Sprintf("ChatRoom-%s", roomName)).
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite)
	return messageBox
}
func createUsageBox() *tview.TextView {
	usageBox := tview.NewTextView().
		SetDynamicColors(true).
		SetText(`[red]/exit[green] - exit | [red]/room <roomname>[green] - switch rooms | [red]/user <username>[green] - change name | [red]/clear[green] - clear chat`)
	usageBox.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("Usage").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite).
		SetBorderPadding(0, 0, 1, 0)
	return usageBox
}

func createPeerBox() *tview.TextView {
	peerBox := tview.NewTextView().
		SetDynamicColors(true)
	peerBox.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("Peers").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite)
	return peerBox
}

func createInputField(username string, cmdChan chan UICommand, msgChan chan string) *tview.InputField {
	input := tview.NewInputField().
		SetLabel(username + " > ").
		SetLabelColor(tcell.ColorGreen).
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack)
	input.
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle("Input").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorWhite).
		SetBorderPadding(0, 0, 1, 0)

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			line := input.GetText()
			if len(line) > 0 {
				if strings.HasPrefix(line, "/") {
					cmdParts := strings.SplitN(line, " ", 2)
					arg := ""
					if len(cmdParts) > 1 {
						arg = cmdParts[1]
					}
					cmdChan <- UICommand{CommandType: cmdParts[0], Argument: arg}
				} else {
					msgChan <- line
				}
				input.SetText("")
			}
		}
	})

	return input
}
