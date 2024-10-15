package pkg

import (
	"context"
	"crypto/rand"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tls "github.com/libp2p/go-libp2p-tls"
	yamux "github.com/libp2p/go-libp2p-yamux"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

// setupHost initializes and configures a libP2P host with various networking and security options,
// including Kademlia DHT, GossipSub, NAT traversal, auto-relay, and connection management.
func setupHost(ctx context.Context) (host.Host, *dht.IpfsDHT, error) {
	// Generate PeerNetwork identity (cryptographic key pair)
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	logrus.Debugln("Generated PeerNetwork Identity.")

	// Configure security, transport, and listener options
	tlsTransport, err := tls.New(prvKey)
	if err != nil {
		return nil, nil, err
	}

	multiAddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	if err != nil {
		return nil, nil, err
	}

	opts := []libp2p.Option{
		libp2p.Identity(prvKey),
		libp2p.Security(tls.ID, tlsTransport),
		libp2p.ListenAddrs(multiAddr),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
		libp2p.ConnectionManager(connmgr.NewConnManager(100, 400, time.Minute)),
		libp2p.NATPortMap(),
		libp2p.EnableAutoRelay(),
	}

	// Add Kademlia DHT setup to libP2P options
	var kadDHT *dht.IpfsDHT
	opts = append(opts, libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		kadDHT = setupKadDHT(ctx, h)
		return kadDHT, nil
	}))

	// Create libP2P host
	libHost, err := libp2p.New(ctx, opts...)
	if err != nil {
		return nil, nil, err
	}

	return libHost, kadDHT, nil
}

// setupKadDHT initializes the Kademlia DHT in server mode with bootstrap peers.
func setupKadDHT(ctx context.Context, nodeHost host.Host) *dht.IpfsDHT {
	kadDHT, err := dht.New(ctx, nodeHost, dht.Mode(dht.ModeServer), dht.BootstrapPeers(dht.GetDefaultBootstrapPeerAddrInfos()...))
	if err != nil {
		logrus.WithError(err).Fatalln("Failed to create Kademlia DHT")
	}
	return kadDHT
}

// setupPubSub initializes a GossipSub-based PubSub system using the given node host and routing discovery.
func setupPubSub(ctx context.Context, nodeHost host.Host, discovery *discovery.RoutingDiscovery) (*pubsub.PubSub, error) {
	pubSubHandler, err := pubsub.NewGossipSub(ctx, nodeHost, pubsub.WithDiscovery(discovery))
	if err != nil {
		return nil, err
	}
	return pubSubHandler, nil
}

// bootstrapDHT bootstraps the Kademlia DHT and connects the host to the default bootstrap peers.
func bootstrapDHT(ctx context.Context, nodeHost host.Host, kadDHT *dht.IpfsDHT) error {
	if err := kadDHT.Bootstrap(ctx); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerInfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func(peerInfo peer.AddrInfo) {
			defer wg.Done()
			if err := nodeHost.Connect(ctx, peerInfo); err == nil {
				logrus.Debugf("Connected to bootstrap peer: %s", peerInfo.ID)
			}
		}(*peerInfo)
	}
	wg.Wait()
	return nil
}
