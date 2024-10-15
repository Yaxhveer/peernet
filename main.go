package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yaxhveer/peernet/pkg"
)

func main() {
	// Command-line flags
	userName := flag.String("user", "user", "Specify username.")
	roomName := flag.String("room", "lobby", "Specify the room to join.")
	discoveryMethod := flag.String("discover", "", "Set peer discovery method ('announce' or 'advertise').")
	enableDebug := flag.Bool("debug", false, "Enable debug logs.")

	// Parse command-line flags
	flag.Parse()

	// Setup logging
	setupLogging(*enableDebug)

	logrus.Info("Starting PeerNet... Please wait for up to 30 seconds.")

	// Initialize P2P Host
	p2pHost, err := initPeerNetworkHost()
	if err != nil {
		logrus.Fatalf("Failed to initialize P2P host: %v", err)
	}
	logrus.Info("P2P network setup complete.")

	// Establish peer discovery and connection
	err = connectToPeers(p2pHost, *discoveryMethod)
	if err != nil {
		logrus.Fatalf("Failed to connect to peers: %v", err)
	}
	logrus.Info("Successfully connected to peers.")

	// Join the room
	chatRoom, err := pkg.JoinChatRoom(p2pHost, *userName, *roomName)
	if err != nil {
		logrus.Fatalf("Failed to join chatroom: %v", err)
	}
	logrus.Infof("Joined chatroom '%s' as user '%s'", chatRoom.RoomName, chatRoom.UserName)

	// Allow time for network setup
	time.Sleep(2 * time.Second)

	// Start UI
	ui := pkg.NewUI(chatRoom)
	if err := ui.Run(); err != nil {
		logrus.Fatalf("Error running chat UI: %v", err)
	}
}

// setupLogging configures the logging level and format.
func setupLogging(enableDebug bool) {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC822,
	})
	logrus.SetOutput(os.Stdout)

	if enableDebug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("Debug mode enabled.")
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

// initP2PHost initializes the P2P network host.
func initPeerNetworkHost() (*pkg.PeerNetwork, error) {
	p2pHost, err := pkg.NewP2P(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error initializing PeerNetwork host: %w", err)
	}
	return p2pHost, nil
}

// connectToPeers handles peer discovery based on the specified method.
func connectToPeers(p2pHost *pkg.PeerNetwork, discoveryMethod string) error {
	switch discoveryMethod {
	case "announce":
		logrus.Debug("Using 'announce' for peer discovery.")
		p2pHost.AnnounceConnect()
	case "advertise":
		logrus.Debug("Using 'advertise' for peer discovery.")
		p2pHost.AdvertiseConnect()
	default:
		logrus.Debug("No discovery method specified, defaulting to 'advertise'.")
		p2pHost.AdvertiseConnect()
	}
	return nil
}
