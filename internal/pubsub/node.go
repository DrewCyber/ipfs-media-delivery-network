package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/atregu/ipfs-publisher/internal/logger"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
)

// Node represents an embedded libp2p PubSub node
type Node struct {
	host      host.Host
	ps        *pubsub.PubSub
	dht       *dht.IpfsDHT
	ctx       context.Context
	cancel    context.CancelFunc
	topic     *pubsub.Topic
	topicName string
	mu        sync.Mutex
	started   bool
}

// Config holds PubSub node configuration
type Config struct {
	Topic          string   // PubSub topic name
	ListenPort     int      // Port to listen on (0 = random)
	BootstrapPeers []string // Bootstrap peer multiaddrs
}

// NewNode creates a new PubSub node
func NewNode(cfg *Config) (*Node, error) {
	ctx, cancel := context.WithCancel(context.Background())

	node := &Node{
		ctx:       ctx,
		cancel:    cancel,
		topicName: cfg.Topic,
	}

	return node, nil
}

// Start initializes and starts the PubSub node
func (n *Node) Start(cfg *Config) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.started {
		return fmt.Errorf("node already started")
	}

	log := logger.Get()
	log.Info("Starting PubSub node...")

	// Create listen address
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.ListenPort)

	// Create libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(listenAddr),
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
	)
	if err != nil {
		return fmt.Errorf("failed to create libp2p host: %w", err)
	}
	n.host = h

	log.Infof("PubSub node started with Peer ID: %s", h.ID())
	log.Infof("Listening on: %v", h.Addrs())

	// Create DHT for peer discovery
	dhtInstance, err := dht.New(n.ctx, h)
	if err != nil {
		h.Close()
		return fmt.Errorf("failed to create DHT: %w", err)
	}
	n.dht = dhtInstance

	// Bootstrap DHT
	if err := dhtInstance.Bootstrap(n.ctx); err != nil {
		h.Close()
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Connect to bootstrap peers
	if err := n.connectBootstrapPeers(cfg.BootstrapPeers); err != nil {
		log.Warnf("Failed to connect to some bootstrap peers: %v", err)
	}

	// Create PubSub instance with GossipSub
	ps, err := pubsub.NewGossipSub(n.ctx, h)
	if err != nil {
		h.Close()
		return fmt.Errorf("failed to create GossipSub: %w", err)
	}
	n.ps = ps

	// Join topic
	topic, err := ps.Join(n.topicName)
	if err != nil {
		h.Close()
		return fmt.Errorf("failed to join topic %s: %w", n.topicName, err)
	}
	n.topic = topic

	log.Infof("Joined PubSub topic: %s", n.topicName)

	// Setup peer discovery
	go n.discoverPeers()

	n.started = true
	return nil
}

// connectBootstrapPeers connects to bootstrap peers
func (n *Node) connectBootstrapPeers(bootstrapPeers []string) error {
	log := logger.Get()

	// Use default IPFS bootstrap peers if none provided
	if len(bootstrapPeers) == 0 {
		// Convert default bootstrap peers to strings
		for _, maddr := range dht.DefaultBootstrapPeers {
			bootstrapPeers = append(bootstrapPeers, maddr.String())
		}
	}

	var wg sync.WaitGroup
	successCount := 0
	mu := sync.Mutex{}

	for _, peerAddr := range bootstrapPeers {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()

			maddr, err := multiaddr.NewMultiaddr(addr)
			if err != nil {
				log.Debugf("Invalid bootstrap peer address %s: %v", addr, err)
				return
			}

			peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Debugf("Failed to parse peer info from %s: %v", addr, err)
				return
			}

			if err := n.host.Connect(n.ctx, *peerInfo); err != nil {
				log.Debugf("Failed to connect to bootstrap peer %s: %v", peerInfo.ID, err)
				return
			}

			mu.Lock()
			successCount++
			mu.Unlock()
			log.Debugf("Connected to bootstrap peer: %s", peerInfo.ID)
		}(peerAddr)
	}

	wg.Wait()
	log.Infof("Connected to %d bootstrap peers", successCount)

	if successCount == 0 {
		return fmt.Errorf("failed to connect to any bootstrap peers")
	}

	return nil
}

// discoverPeers continuously discovers peers on the topic
func (n *Node) discoverPeers() {
	log := logger.Get()

	routingDiscovery := routing.NewRoutingDiscovery(n.dht)
	util.Advertise(n.ctx, routingDiscovery, n.topicName)

	log.Debug("Advertising presence on PubSub topic")

	// Look for peers
	peerChan, err := routingDiscovery.FindPeers(n.ctx, n.topicName)
	if err != nil {
		log.Errorf("Failed to find peers: %v", err)
		return
	}

	for peer := range peerChan {
		if peer.ID == n.host.ID() {
			continue
		}

		log.Debugf("Discovered peer: %s", peer.ID)

		if n.host.Network().Connectedness(peer.ID) != 1 { // Not connected
			if err := n.host.Connect(n.ctx, peer); err != nil {
				log.Debugf("Failed to connect to discovered peer %s: %v", peer.ID, err)
			} else {
				log.Infof("Connected to PubSub peer: %s", peer.ID)
			}
		}
	}
}

// Publish publishes a message to the topic
func (n *Node) Publish(data []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started {
		return fmt.Errorf("node not started")
	}

	if n.topic == nil {
		return fmt.Errorf("topic not joined")
	}

	if err := n.topic.Publish(n.ctx, data); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Subscribe subscribes to the topic and returns a subscription
func (n *Node) Subscribe() (*pubsub.Subscription, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started {
		return nil, fmt.Errorf("node not started")
	}

	if n.topic == nil {
		return nil, fmt.Errorf("topic not joined")
	}

	sub, err := n.topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	return sub, nil
}

// GetPeerCount returns the number of connected peers
func (n *Node) GetPeerCount() int {
	if n.host == nil {
		return 0
	}
	return len(n.host.Network().Peers())
}

// GetTopicPeerCount returns the number of peers on the topic
func (n *Node) GetTopicPeerCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.topic == nil {
		return 0
	}
	return len(n.topic.ListPeers())
}

// GetPeerID returns the node's peer ID
func (n *Node) GetPeerID() string {
	if n.host == nil {
		return ""
	}
	return n.host.ID().String()
}

// GetListenAddresses returns the node's listen addresses
func (n *Node) GetListenAddresses() []string {
	if n.host == nil {
		return nil
	}

	addrs := n.host.Addrs()
	result := make([]string, 0, len(addrs))

	for _, addr := range addrs {
		// Combine address with peer ID
		fullAddr := fmt.Sprintf("%s/p2p/%s", addr.String(), n.host.ID().String())
		result = append(result, fullAddr)
	}

	return result
}

// Stop stops the PubSub node
func (n *Node) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started {
		return nil
	}

	log := logger.Get()
	log.Info("Stopping PubSub node...")

	n.cancel()

	if n.topic != nil {
		n.topic.Close()
	}

	if n.dht != nil {
		n.dht.Close()
	}

	if n.host != nil {
		n.host.Close()
	}

	n.started = false
	log.Info("PubSub node stopped")
	return nil
}
