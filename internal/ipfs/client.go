package ipfs

import (
	"context"
	"io"
)

// AddOptions contains options for adding files to IPFS
type AddOptions struct {
	Pin       bool
	NoCopy    bool
	Chunker   string
	RawLeaves bool
}

// IPNSPublishOptions contains options for IPNS publishing
type IPNSPublishOptions struct {
	Key          string // IPNS key name
	Lifetime     string // Record lifetime (e.g., "24h")
	TTL          string // TTL for the record
	AllowOffline bool   // Allow offline publishing (local only, no DHT)
}

// AddResult contains the result of adding a file to IPFS
type AddResult struct {
	CID  string
	Size uint64
	Name string
}

// IPNSPublishResult contains the result of IPNS publish
type IPNSPublishResult struct {
	Name  string // IPNS name (hash)
	Value string // CID being published
}

// Client defines the interface for IPFS operations
type Client interface {
	// Add uploads a file to IPFS and returns its CID
	Add(ctx context.Context, reader io.Reader, filename string, opts AddOptions) (*AddResult, error)

	// Cat retrieves content from IPFS by CID
	Cat(ctx context.Context, cid string) (io.ReadCloser, error)

	// Pin pins content in IPFS
	Pin(ctx context.Context, cid string) error

	// Unpin unpins content from IPFS
	Unpin(ctx context.Context, cid string) error

	// PublishIPNS publishes a CID to IPNS
	PublishIPNS(ctx context.Context, cid string, opts IPNSPublishOptions) (*IPNSPublishResult, error)

	// ResolveIPNS resolves an IPNS name to a CID
	ResolveIPNS(ctx context.Context, name string) (string, error)

	// IsAvailable checks if the IPFS node is reachable
	IsAvailable(ctx context.Context) error

	// Close closes the client and releases resources
	Close() error
}
