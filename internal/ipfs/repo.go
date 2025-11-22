package ipfs

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
)

// InitializeRepo creates and initializes a new IPFS repository at the given path
func InitializeRepo(repoPath string, swarmPort, apiPort, gatewayPort int) error {
	// Expand home directory if needed
	if len(repoPath) > 0 && repoPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		repoPath = filepath.Join(home, repoPath[1:])
	}

	// Check if repo already exists
	if fsrepo.IsInitialized(repoPath) {
		return nil // Already initialized
	}

	// Create the directory
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Create default configuration
	cfg, err := config.Init(os.Stdout, 2048)
	if err != nil {
		return fmt.Errorf("failed to create default config: %w", err)
	}

	// Don't modify datastore - use defaults from config.Init()
	// The default flatfs should work with the plugins we load

	// Customize ports
	cfg.Addresses.Swarm = []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", swarmPort),
		fmt.Sprintf("/ip6/::/tcp/%d", swarmPort),
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", swarmPort),
		fmt.Sprintf("/ip6/::/udp/%d/quic-v1", swarmPort),
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1/webtransport", swarmPort),
		fmt.Sprintf("/ip6/::/udp/%d/quic-v1/webtransport", swarmPort),
	}
	cfg.Addresses.API = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)}
	cfg.Addresses.Gateway = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)}

	// Initialize the repository
	if err := fsrepo.Init(repoPath, cfg); err != nil {
		return fmt.Errorf("failed to initialize repo: %w", err)
	}

	return nil
}

// CheckPortAvailable checks if a TCP port is available for use
func CheckPortAvailable(port int) error {
	// Try to listen on the port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is not available: %w", port, err)
	}
	listener.Close()
	return nil
}

// CheckAllPortsAvailable verifies all required ports are available
func CheckAllPortsAvailable(swarmPort, apiPort, gatewayPort int) error {
	ports := map[string]int{
		"swarm":   swarmPort,
		"API":     apiPort,
		"gateway": gatewayPort,
	}

	for name, port := range ports {
		if err := CheckPortAvailable(port); err != nil {
			return fmt.Errorf("%s port %d is already in use. Please check if another IPFS node is running or change ports in config", name, port)
		}
	}

	return nil
}

// OpenRepo opens an existing IPFS repository
func OpenRepo(repoPath string) (repo.Repo, error) {
	// Expand home directory if needed
	if len(repoPath) > 0 && repoPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		repoPath = filepath.Join(home, repoPath[1:])
	}

	if !fsrepo.IsInitialized(repoPath) {
		return nil, fmt.Errorf("repository not initialized at %s", repoPath)
	}

	r, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	return r, nil
}

// CloseRepo safely closes an IPFS repository
func CloseRepo(r repo.Repo) error {
	if r != nil {
		return r.Close()
	}
	return nil
}
