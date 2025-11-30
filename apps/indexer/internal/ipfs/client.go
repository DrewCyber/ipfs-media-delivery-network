package ipfs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/atregu/ipfs-indexer/internal/config"
	"github.com/atregu/ipfs-indexer/internal/logger"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	// Import plugins
	_ "github.com/ipfs/kubo/plugin/plugins/badgerds"
	_ "github.com/ipfs/kubo/plugin/plugins/flatfs"
	_ "github.com/ipfs/kubo/plugin/plugins/levelds"
)

// Client represents an IPFS client interface
type Client struct {
	node    *core.IpfsNode
	api     iface.CoreAPI
	repo    repo.Repo
	cfg     *config.EmbeddedIPFSConfig
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
	pubsub  *pubsub.PubSub
}

var initPluginsOnce sync.Once
var initPluginsErr error

func setupPlugins() error {
	initPluginsOnce.Do(func() {
		plugins, err := loader.NewPluginLoader("")
		if err != nil {
			initPluginsErr = fmt.Errorf("failed to create plugin loader: %w", err)
			return
		}

		if err := plugins.Initialize(); err != nil {
			initPluginsErr = fmt.Errorf("failed to initialize plugins: %w", err)
			return
		}

		if err := plugins.Inject(); err != nil {
			initPluginsErr = fmt.Errorf("failed to inject plugins: %w", err)
			return
		}
	})

	return initPluginsErr
}

// NewClient creates a new IPFS client
func NewClient(cfg *config.EmbeddedIPFSConfig) (*Client, error) {
	log := logger.Get()

	// Initialize plugins
	if err := setupPlugins(); err != nil {
		return nil, err
	}

	// Check port availability
	log.Info("Checking port availability...")
	if err := CheckAllPortsAvailable(cfg.SwarmPort, cfg.APIPort, cfg.GatewayPort); err != nil {
		return nil, err
	}

	// Initialize repository
	log.Infof("Initializing repository at %s...", cfg.RepoPath)
	if err := InitializeRepo(cfg.RepoPath, cfg.SwarmPort, cfg.APIPort, cfg.GatewayPort); err != nil {
		return nil, fmt.Errorf("failed to initialize repo: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	return client, nil
}

// Start starts the embedded IPFS node
func (c *Client) Start() error {
	if c.started {
		return fmt.Errorf("node already started")
	}

	log := logger.Get()
	log.Info("Starting embedded IPFS node...")

	// Open the repository
	repo, err := OpenRepo(c.cfg.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}
	c.repo = repo

	// Build the IPFS node
	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption,
		Repo:    repo,
		ExtraOpts: map[string]bool{
			"pubsub": true,
		},
	}

	node, err := core.NewNode(c.ctx, nodeOptions)
	if err != nil {
		CloseRepo(repo)
		return fmt.Errorf("failed to create IPFS node: %w", err)
	}
	c.node = node

	// Get PubSub instance
	c.pubsub = node.PubSub

	// Create CoreAPI
	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		node.Close()
		CloseRepo(repo)
		return fmt.Errorf("failed to create CoreAPI: %w", err)
	}
	c.api = api

	c.started = true

	// Wait for node to be ready
	time.Sleep(2 * time.Second)

	// Log node information
	id := c.node.Identity.String()
	log.Infof("Embedded IPFS node started successfully. Peer ID: %s", id)

	// Log swarm addresses
	addrs, err := c.api.Swarm().ListenAddrs(c.ctx)
	if err != nil {
		log.Warnf("Failed to get swarm addresses: %v", err)
	} else {
		log.Infof("Listening on %d addresses", len(addrs))
	}

	return nil
}

// GetPubSub returns the PubSub instance
func (c *Client) GetPubSub() *pubsub.PubSub {
	return c.pubsub
}

// GetPeerID returns the peer ID of the node
func (c *Client) GetPeerID() peer.ID {
	if c.node != nil {
		return c.node.Identity
	}
	return ""
}

// ResolveIPNS resolves an IPNS name to an IPFS CID
func (c *Client) ResolveIPNS(ctx context.Context, ipnsName string) (string, error) {
	if !c.started {
		return "", fmt.Errorf("node not started")
	}

	// Ensure name has /ipns/ prefix
	if !strings.HasPrefix(ipnsName, "/ipns/") {
		ipnsName = "/ipns/" + ipnsName
	}

	// Parse the IPNS path
	p, err := path.NewPath(ipnsName)
	if err != nil {
		return "", fmt.Errorf("failed to parse IPNS path: %w", err)
	}

	// Resolve the name
	resolved, err := c.api.Name().Resolve(ctx, p.String())
	if err != nil {
		return "", fmt.Errorf("failed to resolve IPNS: %w", err)
	}

	// Extract the CID from the resolved path
	resolvedPath := resolved.String()
	if strings.HasPrefix(resolvedPath, "/ipfs/") {
		return strings.TrimPrefix(resolvedPath, "/ipfs/"), nil
	}

	return resolvedPath, nil
}

// Cat retrieves file content from IPFS by CID
func (c *Client) Cat(ctx context.Context, cid string) (io.ReadCloser, error) {
	if !c.started {
		return nil, fmt.Errorf("node not started")
	}

	// Parse the path
	p, err := path.NewPath("/ipfs/" + cid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	// Get the file
	node, err := c.api.Unixfs().Get(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	// Convert files.Node to io.ReadCloser
	file, ok := node.(files.File)
	if !ok {
		return nil, fmt.Errorf("node is not a file")
	}

	return file, nil
}

// Subscribe subscribes to a PubSub topic
func (c *Client) Subscribe(ctx context.Context, topic string) (*pubsub.Subscription, error) {
	if !c.started || c.pubsub == nil {
		return nil, fmt.Errorf("node not started or pubsub not available")
	}

	sub, err := c.pubsub.Subscribe(topic)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	return sub, nil
}

// Close gracefully shuts down the node
func (c *Client) Close() error {
	if !c.started {
		return nil
	}

	log := logger.Get()
	log.Info("Shutting down embedded IPFS node...")

	c.started = false

	// Cancel context
	if c.cancel != nil {
		c.cancel()
	}

	// Close the node
	if c.node != nil {
		if err := c.node.Close(); err != nil {
			log.Errorf("Error closing IPFS node: %v", err)
		}
	}

	// Close the repository
	if c.repo != nil {
		if err := CloseRepo(c.repo); err != nil {
			log.Errorf("Error closing repo: %v", err)
		}
	}

	log.Info("Embedded IPFS node shut down successfully")
	return nil
}
