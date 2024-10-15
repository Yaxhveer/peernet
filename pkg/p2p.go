package pkg

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/sirupsen/logrus"
)

const SERVICE = "peernet"

// PeerNetwork represents a structure that encapsulates P2P communication components.
type PeerNetwork struct {
	Ctx       context.Context
	Host      host.Host
	KadDHT    *dht.IpfsDHT
	Discovery *discovery.RoutingDiscovery
	PubSub    *pubsub.PubSub
}

// NewP2P initializes a new PeerNetwork instance with a Kademlia DHT and PubSub service.
func NewP2P(ctx context.Context) (*PeerNetwork, error) {
	// Setup the host and KadDHT
	nodehost, kaddht, err := setupHost(ctx)
	if err != nil {
		return nil, err
	}
	logrus.Debugln("Created the PeerNetwork Host and Kademlia DHT")

	// Bootstrap the KadDHT
	if err := bootstrapDHT(ctx, nodehost, kaddht); err != nil {
		return nil, err
	}
	logrus.Debugln("Bootstrapped the Kademlia DHT")

	// Create peer discovery service
	routingDiscovery := discovery.NewRoutingDiscovery(kaddht)
	logrus.Debugln("Created the Peer Discovery Service")

	// Create a PubSub handler
	pubsubHandler, err := setupPubSub(ctx, nodehost, routingDiscovery)
	if err != nil {
		return nil, err
	}
	logrus.Debugln("Created the PubSub Handler")

	return &PeerNetwork{
		Ctx:       ctx,
		Host:      nodehost,
		KadDHT:    kaddht,
		Discovery: routingDiscovery,
		PubSub:    pubsubHandler,
	}, nil
}
