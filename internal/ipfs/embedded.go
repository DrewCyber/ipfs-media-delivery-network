package ipfs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	config "github.com/atregu/ipfs-publisher/internal/config"
	"github.com/atregu/ipfs-publisher/internal/logger"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo"

	// Import plugins - they are preloaded automatically by kubo's plugin/loader/preload.go
	_ "github.com/ipfs/kubo/plugin/plugins/badgerds"
	_ "github.com/ipfs/kubo/plugin/plugins/flatfs"
	_ "github.com/ipfs/kubo/plugin/plugins/levelds"
)

// EmbeddedClient implements the Client interface using an embedded IPFS node
type EmbeddedClient struct {
	node    *core.IpfsNode
	api     iface.CoreAPI
	repo    repo.Repo
	cfg     *config.EmbeddedIPFSConfig
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

var initPluginsOnce sync.Once
var initPluginsErr error

func setupPlugins() error {
	initPluginsOnce.Do(func() {
		// NewPluginLoader with empty string will use only preloaded plugins
		// and won't try to load from a plugins directory
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

// NewEmbeddedClient creates a new embedded IPFS client
func NewEmbeddedClient(cfg *config.EmbeddedIPFSConfig) (*EmbeddedClient, error) {
	log := logger.Get()

	// Initialize plugins once (using preloaded plugins from init())
	if err := setupPlugins(); err != nil {
		return nil, err
	}

	// Check port availability before initializing
	log.Info("Checking port availability...")
	if err := CheckAllPortsAvailable(cfg.SwarmPort, cfg.APIPort, cfg.GatewayPort); err != nil {
		return nil, err
	}

	// Initialize repository if it doesn't exist
	log.Infof("Initializing repository at %s...", cfg.RepoPath)
	if err := InitializeRepo(cfg.RepoPath, cfg.SwarmPort, cfg.APIPort, cfg.GatewayPort); err != nil {
		return nil, fmt.Errorf("failed to initialize repo: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &EmbeddedClient{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	return client, nil
}

// Start starts the embedded IPFS node
func (c *EmbeddedClient) Start() error {
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
	id, err := c.GetID()
	if err != nil {
		log.Warnf("Failed to get node ID: %v", err)
	} else {
		log.Infof("Embedded IPFS node started successfully. Peer ID: %s", id)
	}

	// Log swarm addresses
	addrs, err := c.api.Swarm().ListenAddrs(c.ctx)
	if err != nil {
		log.Warnf("Failed to get swarm addresses: %v", err)
	} else {
		log.Infof("Listening on %d addresses", len(addrs))
	}

	return nil
}

// Add uploads a file to IPFS
func (c *EmbeddedClient) Add(ctx context.Context, reader io.Reader, filename string, opts AddOptions) (*AddResult, error) {
	if !c.started {
		return nil, fmt.Errorf("node not started")
	}

	// Read all data from reader into memory
	// This is necessary because files.NewReaderFile expects a ReadSeeker
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// Create a files.Node from the data
	fileNode := files.NewBytesFile(data)

	// Build add options
	pinName := ""
	if opts.Pin {
		pinName = filename
	}

	addOpts := []options.UnixfsAddOption{
		options.Unixfs.Pin(opts.Pin, pinName),
		options.Unixfs.RawLeaves(opts.RawLeaves),
	}

	// Add chunker if specified
	if opts.Chunker != "" {
		addOpts = append(addOpts, options.Unixfs.Chunker(opts.Chunker))
	}

	// Add the file
	p, err := c.api.Unixfs().Add(ctx, fileNode, addOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to add file: %w", err)
	}

	result := &AddResult{
		CID:  p.RootCid().String(),
		Name: filename,
		Size: uint64(len(data)),
	}

	return result, nil
}

// Cat retrieves file content from IPFS
func (c *EmbeddedClient) Cat(ctx context.Context, cid string) (io.ReadCloser, error) {
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

// Pin pins content by CID
func (c *EmbeddedClient) Pin(ctx context.Context, cid string) error {
	if !c.started {
		return fmt.Errorf("node not started")
	}

	// Parse the path
	p, err := path.NewPath("/ipfs/" + cid)
	if err != nil {
		return fmt.Errorf("failed to parse path: %w", err)
	}

	// Pin the content
	if err := c.api.Pin().Add(ctx, p); err != nil {
		return fmt.Errorf("failed to pin: %w", err)
	}

	return nil
}

// Unpin unpins content by CID
func (c *EmbeddedClient) Unpin(ctx context.Context, cid string) error {
	if !c.started {
		return fmt.Errorf("node not started")
	}

	// Parse the path
	p, err := path.NewPath("/ipfs/" + cid)
	if err != nil {
		return fmt.Errorf("failed to parse path: %w", err)
	}

	// Unpin the content
	if err := c.api.Pin().Rm(ctx, p); err != nil {
		return fmt.Errorf("failed to unpin: %w", err)
	}

	return nil
}

// PublishIPNS publishes an IPFS path to IPNS
func (c *EmbeddedClient) PublishIPNS(ctx context.Context, cid string, opts IPNSPublishOptions) (*IPNSPublishResult, error) {
	if !c.started {
		return nil, fmt.Errorf("node not started")
	}

	// Parse the path
	p, err := path.NewPath("/ipfs/" + cid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	// Parse lifetime and TTL
	var publishOpts []options.NamePublishOption

	if opts.Key != "" {
		publishOpts = append(publishOpts, options.Name.Key(opts.Key))
	}

	if opts.Lifetime != "" {
		if d, err := time.ParseDuration(opts.Lifetime); err == nil {
			publishOpts = append(publishOpts, options.Name.ValidTime(d))
		}
	}

	if opts.TTL != "" {
		if d, err := time.ParseDuration(opts.TTL); err == nil {
			publishOpts = append(publishOpts, options.Name.TTL(d))
		}
	}

	// Allow offline publishing if requested (local only, no DHT)
	if opts.AllowOffline {
		publishOpts = append(publishOpts, options.Name.AllowOffline(true))
	}

	// Publish
	entry, err := c.api.Name().Publish(ctx, p, publishOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to publish IPNS: %w", err)
	}

	result := &IPNSPublishResult{
		Name:  entry.String(),
		Value: p.String(),
	}

	return result, nil
}

// ResolveIPNS resolves an IPNS name to an IPFS path
func (c *EmbeddedClient) ResolveIPNS(ctx context.Context, name string) (string, error) {
	if !c.started {
		return "", fmt.Errorf("node not started")
	}

	// Ensure name has /ipns/ prefix
	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	// Parse the IPNS path
	p, err := path.NewPath(name)
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

// IsAvailable checks if the embedded node is running
func (c *EmbeddedClient) IsAvailable(ctx context.Context) error {
	if !c.started || c.node == nil {
		return fmt.Errorf("node not started")
	}

	// Try to get node ID as a simple health check
	_, err := c.GetID()
	if err != nil {
		return fmt.Errorf("node not available: %w", err)
	}

	return nil
}

// GetVersion returns the IPFS version (for embedded, return kubo version)
func (c *EmbeddedClient) GetVersion() (string, error) {
	if !c.started {
		return "", fmt.Errorf("node not started")
	}

	// Return a static version string for embedded node
	return "kubo/0.38.2 (embedded)", nil
}

// GetID returns the peer ID of the embedded node
func (c *EmbeddedClient) GetID() (string, error) {
	if !c.started || c.node == nil {
		return "", fmt.Errorf("node not started")
	}

	return c.node.Identity.String(), nil
}

// Close gracefully shuts down the embedded node
func (c *EmbeddedClient) Close() error {
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
