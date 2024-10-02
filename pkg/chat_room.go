package pkg

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// ChatRoom represents a PubSub-based chat room.
type ChatRoom struct {
	Host     *PeerNetwork     // PeerNetwork host instance
	Inbound  chan chatMessage // Incoming messages channel
	Outbound chan string      // Outgoing messages channel
	Logs     chan chatLog     // Chat log messages channel

	RoomName string  // Name of the chat room
	UserName string  // Name of the user in the chat room
	selfID   peer.ID // Host ID of the peer

	psCtx    context.Context      // PubSub context for managing lifecycle
	psCancel context.CancelFunc   // PubSub cancellation function
	psTopic  *pubsub.Topic        // PubSub topic for the chat room
	psSub    *pubsub.Subscription // PubSub subscription for the topic
}

// chatMessage represents a single chat message.
type chatMessage struct {
	Message    string `json:"message"`
	SenderID   string `json:"senderid"`
	SenderName string `json:"sendername"`
}

// chatLog represents a log message for the chat room.
type chatLog struct {
	Prefix string
	Msg    string
}

// JoinChatRoom creates and returns a new ChatRoom instance.
func JoinChatRoom(p2pHost *PeerNetwork, username, roomName string) (*ChatRoom, error) {
	// Join the PubSub topic for the room
	topic, err := p2pHost.PubSub.Join(fmt.Sprintf("room-peerchat-%s", roomName))
	if err != nil {
		return nil, err
	}

	// Subscribe to the PubSub topic
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	// Create a cancellable context
	psCtx, cancel := context.WithCancel(context.Background())

	// Initialize a ChatRoom instance
	chatRoom := &ChatRoom{
		Host:     p2pHost,
		Inbound:  make(chan chatMessage, 1),
		Outbound: make(chan string, 1),
		Logs:     make(chan chatLog, 1),
		RoomName: roomName,
		UserName: username,
		selfID:   p2pHost.Host.ID(),
		psCtx:    psCtx,
		psCancel: cancel,
		psTopic:  topic,
		psSub:    sub,
	}

	// Start loops for subscription and publishing
	go chatRoom.subscribeLoop()
	go chatRoom.publishLoop()

	return chatRoom, nil
}

// publishLoop handles publishing outbound chat messages to the PubSub topic.
func (cr *ChatRoom) publishLoop() {
	for {
		select {
		case <-cr.psCtx.Done():
			return
		case message := <-cr.Outbound:
			// Create a chatMessage instance
			chatMsg := chatMessage{
				Message:    message,
				SenderID:   cr.selfID.Pretty(),
				SenderName: cr.UserName,
			}

			// Serialize the message to JSON
			msgBytes, err := json.Marshal(chatMsg)
			if err != nil {
				cr.Logs <- chatLog{Prefix: "puberr", Msg: "failed to marshal JSON"}
				continue
			}

			// Publish the message to the PubSub topic
			if err := cr.psTopic.Publish(cr.psCtx, msgBytes); err != nil {
				cr.Logs <- chatLog{Prefix: "puberr", Msg: "failed to publish message"}
			}
		}
	}
}

// subscribeLoop handles reading inbound messages from the PubSub subscription.
func (cr *ChatRoom) subscribeLoop() {
	for {
		select {
		case <-cr.psCtx.Done():
			close(cr.Inbound)
			return
		default:
			// Read the next message from the PubSub subscription
			msg, err := cr.psSub.Next(cr.psCtx)
			if err != nil {
				cr.Logs <- chatLog{Prefix: "suberr", Msg: "subscription closed"}
				close(cr.Inbound)
				return
			}

			// Ignore messages sent by self
			if msg.ReceivedFrom == cr.selfID {
				continue
			}

			// Deserialize the message data into chatMessage
			var chatMsg chatMessage
			if err := json.Unmarshal(msg.Data, &chatMsg); err != nil {
				cr.Logs <- chatLog{Prefix: "suberr", Msg: "failed to unmarshal JSON"}
				continue
			}

			// Send the message to the inbound channel
			cr.Inbound <- chatMsg
		}
	}
}

// PeerList returns a list of peer IDs connected to the PubSub topic.
func (cr *ChatRoom) PeerList() []peer.ID {
	return cr.psTopic.ListPeers()
}

// Exit gracefully leaves the chat room by canceling the subscription and closing the topic.
func (cr *ChatRoom) Exit() {
	defer cr.psCancel()
	cr.psSub.Cancel()
	cr.psTopic.Close()
}

// UpdateUser updates the username for the chat room user.
func (cr *ChatRoom) UpdateUser(newUsername string) {
	cr.UserName = newUsername
}
