package ipfs

import (
	"context"
	"fmt"
	"io"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

// ExternalClient implements the Client interface for external IPFS nodes via HTTP API
type ExternalClient struct {
	shell   *shell.Shell
	apiURL  string
	timeout time.Duration
}

// NewExternalClient creates a new external IPFS client
func NewExternalClient(apiURL string, timeout time.Duration) (*ExternalClient, error) {
	sh := shell.NewShell(apiURL)

	// Set timeout
	sh.SetTimeout(timeout)

	return &ExternalClient{
		shell:   sh,
		apiURL:  apiURL,
		timeout: timeout,
	}, nil
}

// Add uploads a file to IPFS and returns its CID
func (c *ExternalClient) Add(ctx context.Context, reader io.Reader, filename string, opts AddOptions) (*AddResult, error) {
	// Build add options
	addOpts := []shell.AddOpts{
		shell.Pin(opts.Pin), // Explicitly set pin option
	}

	if opts.RawLeaves {
		addOpts = append(addOpts, shell.RawLeaves(true))
	}

	// Note: NoCopy and Chunker options are not exposed in go-ipfs-api v0.7.0
	// They would need to be added via the underlying HTTP request if needed

	// Add file to IPFS
	cid, err := c.shell.Add(reader, addOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to add file to IPFS: %w", err)
	}

	return &AddResult{
		CID:  cid,
		Name: filename,
	}, nil
}

// Cat retrieves content from IPFS by CID
func (c *ExternalClient) Cat(ctx context.Context, cid string) (io.ReadCloser, error) {
	reader, err := c.shell.Cat(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to cat CID %s: %w", cid, err)
	}
	return reader, nil
}

// Pin pins content in IPFS
func (c *ExternalClient) Pin(ctx context.Context, cid string) error {
	if err := c.shell.Pin(cid); err != nil {
		return fmt.Errorf("failed to pin CID %s: %w", cid, err)
	}
	return nil
}

// Unpin unpins content from IPFS
func (c *ExternalClient) Unpin(ctx context.Context, cid string) error {
	if err := c.shell.Unpin(cid); err != nil {
		return fmt.Errorf("failed to unpin CID %s: %w", cid, err)
	}
	return nil
}

// PublishIPNS publishes a CID to IPNS
func (c *ExternalClient) PublishIPNS(ctx context.Context, cid string, opts IPNSPublishOptions) (*IPNSPublishResult, error) {
	// Use PublishWithDetails for more control
	// Default lifetime: 24h, TTL: 0 (use default), resolve: true
	lifetime := 24 * time.Hour
	if opts.Lifetime != "" {
		if d, err := time.ParseDuration(opts.Lifetime); err == nil {
			lifetime = d
		}
	}

	ttl := time.Duration(0)
	if opts.TTL != "" {
		if d, err := time.ParseDuration(opts.TTL); err == nil {
			ttl = d
		}
	}

	resp, err := c.shell.PublishWithDetails(cid, opts.Key, lifetime, ttl, true)
	if err != nil {
		return nil, fmt.Errorf("failed to publish to IPNS: %w", err)
	}

	return &IPNSPublishResult{
		Name:  resp.Name,
		Value: resp.Value,
	}, nil
}

// ResolveIPNS resolves an IPNS name to a CID
func (c *ExternalClient) ResolveIPNS(ctx context.Context, name string) (string, error) {
	path, err := c.shell.Resolve(name)
	if err != nil {
		return "", fmt.Errorf("failed to resolve IPNS name %s: %w", name, err)
	}
	return path, nil
}

// IsAvailable checks if the IPFS node is reachable
func (c *ExternalClient) IsAvailable(ctx context.Context) error {
	// Try to get node ID as a health check
	_, err := c.shell.ID()
	if err != nil {
		return fmt.Errorf("IPFS node not available: %w", err)
	}
	return nil
}

// Close closes the client and releases resources
func (c *ExternalClient) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}

// GetVersion returns the IPFS version information
func (c *ExternalClient) GetVersion() (string, error) {
	version, _, err := c.shell.Version()
	if err != nil {
		return "", fmt.Errorf("failed to get IPFS version: %w", err)
	}
	return version, nil
}

// GetID returns the IPFS node ID
func (c *ExternalClient) GetID() (string, error) {
	id, err := c.shell.ID()
	if err != nil {
		return "", fmt.Errorf("failed to get IPFS node ID: %w", err)
	}
	return id.ID, nil
}
