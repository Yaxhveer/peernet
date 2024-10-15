package pkg

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/sirupsen/logrus"
)

// AdvertiseConnect advertises the PeerChat service and connects to peers.
func (p *PeerNetwork) AdvertiseConnect() error {
	ttl, err := p.Discovery.Advertise(p.Ctx, SERVICE)
	if err != nil {
		return err
	}
	logrus.Debugf("Advertised PeerChat Service, TTL: %s", ttl)

	// Allow time for the advertisement to propagate
	time.Sleep(5 * time.Second)

	peerChan, err := p.Discovery.FindPeers(p.Ctx, SERVICE)
	if err != nil {
		return err
	}

	go handlePeerDiscovery(p.Host, peerChan)
	return nil
}

// AnnounceConnect announces the PeerChat service CID and connects to peers.
func (p *PeerNetwork) AnnounceConnect() error {
	// Generate the Service CID
	cidValue, err := generateCID(SERVICE)
	if err != nil {
		return err
	}

	// Announce that this host can provide the service
	if err := p.KadDHT.Provide(p.Ctx, cidValue, true); err != nil {
		return err
	}
	logrus.Debugln("Announced the PeerChat Service")
	time.Sleep(5 * time.Second)

	// Discover other providers for the service CID
	peerChan := p.KadDHT.FindProvidersAsync(p.Ctx, cidValue, 0)
	go handlePeerDiscovery(p.Host, peerChan)
	return nil
}

// generateCID creates a CID (Content Identifier) from a given name by hashing it with SHA-256
// and encoding it as a multihash.
func generateCID(name string) (cid.Cid, error) {
	hash := sha256.Sum256([]byte(name))
	finalHash := append([]byte{0x12, 0x20}, hash[:]...) // Prefix with SHA-256 identifier

	multiHash, err := multihash.Encode(finalHash, multihash.SHA2_256)
	if err != nil {
		return cid.Undef, err
	}

	newCID := cid.NewCidV1(cid.Raw, multiHash)
	return newCID, nil
}

// handlePeerDiscovery listens on a peer channel for discovered peers and connects to them.
func handlePeerDiscovery(nodeHost host.Host, peerChan <-chan peer.AddrInfo) {
	for peer := range peerChan {
		if peer.ID != nodeHost.ID() {
			nodeHost.Connect(context.Background(), peer)
		}
	}
}
