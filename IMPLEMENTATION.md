# IPFS Media Collection Publisher

A Go application for automatic publishing of media collections to IPFS with announcement via Pubsub.

## Current Status: Phase 4 Complete ✓

### Implemented Features

**Phase 1: Basic structure and configuration** ✅
- ✅ Go module initialization with project structure
- ✅ YAML configuration loading with IPFS mode selection (external/embedded)
- ✅ Structured logging with file rotation and console output
- ✅ Lock file mechanism to prevent multiple instances
- ✅ CLI with flags support (--help, --version, --config, --ipfs-mode, etc.)
- ✅ Configuration validation (ports, paths, IPFS mode)
- ✅ Graceful shutdown with signal handling

**Phase 2: External IPFS client and basic operations** ✅
- ✅ IPFS Client interface for abstraction
- ✅ External IPFS HTTP API client implementation
- ✅ File upload to IPFS with options (pin, raw-leaves)
- ✅ IPNS publish and resolve operations
- ✅ Connection check with --check-ipfs flag
- ✅ Test upload command with --test-upload flag
- ✅ IPNS test command with --test-ipns flag
- ✅ Version and node ID retrieval

**Phase 3: Embedded IPFS node** ✅
- ✅ Embedded IPFS node implementation using kubo v0.38.2 core
- ✅ Plugin system integration (flatfs, levelds, badgerds datastores)
- ✅ Repository initialization and management
- ✅ Custom port configuration (swarm, API, gateway)
- ✅ Port availability checking before startup
- ✅ Full IPFS operations support (add, pin, IPNS publish/resolve)
- ✅ Repository persistence between runs
- ✅ Graceful node shutdown

**Phase 4: PubSub announcements** ✅
- ✅ PubSub message format with version, IPNS, collection size, timestamp
- ✅ Ed25519 message signing and verification
- ✅ Standalone libp2p PubSub node (separate from IPFS node)
- ✅ GossipSub protocol implementation
- ✅ DHT integration for peer discovery
- ✅ Bootstrap peer connection (uses default IPFS bootstrap peers)
- ✅ Message publishing to configurable topic
- ✅ Test command with --test-pubsub flag
- ✅ JSON serialization/deserialization
- ✅ Message validation with timestamp drift check

### Project Structure

```
ipfs-media-delivery-network/
├── cmd/
│   └── ipfs-publisher/
│       └── main.go           # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── ipfs/
│   │   ├── client.go         # IPFS client interface
│   │   ├── external.go       # External IPFS HTTP API client
│   │   ├── embedded.go       # Embedded IPFS node implementation (kubo v0.38.2)
│   │   └── repo.go           # Repository initialization and management
│   ├── pubsub/
│   │   ├── message.go        # PubSub message format with signing/verification
│   │   ├── node.go           # Standalone libp2p PubSub node
│   │   └── publisher.go      # Message publisher with periodic announcements
│   ├── logger/
│   │   └── logger.go         # Logging system
│   └── lockfile/
│       └── lockfile.go       # Lock file management
├── config.yaml               # Sample configuration
├── go.mod                    # Go module definition
├── ipfs-publisher           # Compiled binary
├── README.md                 # User documentation
└── IMPLEMENTATION.md         # This file
```

## Building

```bash
go build -o ipfs-publisher ./cmd/ipfs-publisher
```

## Usage

### Display Help
```bash
./ipfs-publisher --help
```

### Display Version
```bash
./ipfs-publisher --version
```

### Initialize Configuration
```bash
./ipfs-publisher --init
```

### Run with Configuration
```bash
./ipfs-publisher --config ./config.yaml
```

### Override IPFS Mode
```bash
./ipfs-publisher --ipfs-mode embedded
```

### Check IPFS Connection
```bash
./ipfs-publisher --check-ipfs
```

### Test PubSub Announcements
```bash
./ipfs-publisher --test-pubsub
```

### Upload Test File
```bash
./ipfs-publisher --test-upload /path/to/file.mp3
```

### Test IPNS Operations
```bash
./ipfs-publisher --test-ipns
```

## Configuration

The application uses a YAML configuration file. Example:

```yaml
# IPFS node configuration
ipfs:
  mode: "external"  # or "embedded"
  
  external:
    api_url: "http://localhost:5001"
    timeout: 300
    
  embedded:
    repo_path: "~/.ipfs_publisher/ipfs-repo"
    swarm_port: 4002
    api_port: 5002
    gateway_port: 8081

# Directories to monitor
directories:
  - "~/test-media"

# File extensions to process
extensions:
  - "mp3"
  - "mp4"
  - "mkv"

# Logging
logging:
  level: "info"
  file: "~/.ipfs_publisher/logs/app.log"
  console: true
```

## Testing Phase 1

All Phase 1 tests pass:

1. ✅ **Version flag**: `./ipfs-publisher --version`
2. ✅ **Help flag**: `./ipfs-publisher --help`
3. ✅ **Run with config**: Application starts and reads configuration
4. ✅ **Lock file check**: Second instance cannot start
5. ✅ **Logging**: Logs written to file and console
6. ✅ **Config validation**: Invalid IPFS mode rejected
7. ✅ **Graceful shutdown**: Ctrl+C handled properly

## Testing Phase 2

All Phase 2 tests pass:

1. ✅ **Check IPFS connection**: Successfully connects to external node
   ```bash
   ./ipfs-publisher --check-ipfs
   # Output: Version and Node ID displayed
   ```

2. ✅ **Upload small file**: 43KB MP3 file uploaded successfully
   ```bash
   ./ipfs-publisher --test-upload test-media/winamp-it-really-whips-the-llamas-ass.mp3
   # CID: bafkreid3cyrzhkewyf6pd4eqb2ughbaxtokpuwi7xeabgxk46yo6qerwya
   ```

3. ✅ **Upload large file**: 12MB MP3 file uploaded successfully
   ```bash
   ./ipfs-publisher --test-upload test-media/Prodigy_-_Smak_My_Bitch_Up.mp3
   # CID: QmTDWHWuNoVK1pVPooLWsjUEjaYwRRwgmN22prRFd5yyPF
   ```

4. ✅ **Pinning works**: Files verified as pinned
   ```bash
   ipfs pin ls | grep QmTDWHWuNoVK1pVPooLWsjUEjaYwRRwgmN22prRFd5yyPF
   # Output: QmTDWHWuNoVK1pVPooLWsjUEjaYwRRwgmN22prRFd5yyPF recursive
   ```

5. ✅ **IPNS operations**: Publish and resolve working
   ```bash
   ./ipfs-publisher --test-ipns
   # Successfully published to IPNS and resolved back to CID
   ```

## Testing Phase 3

All Phase 3 tests pass:

1. ✅ **Embedded node startup**: Node starts successfully with custom ports
   ```bash
   ./ipfs-publisher --ipfs-mode embedded --check-ipfs
   # Output: Peer ID: QmNYH7Z17TCKkwGf45H5qxbRjjbgEmT42EbZM37uasLoYb
   # Listening on 13 addresses
   ```

2. ✅ **File upload with embedded node**: 33 byte test file uploaded
   ```bash
   ./ipfs-publisher --ipfs-mode embedded --test-upload test.mp3
   # CID: bafkreifddhf4n3f64dknxbpfrp7bbt5luzg643mtmzf5bwde6wmmizwuae
   # Pinned: true
   ```

3. ✅ **IPNS with embedded node**: Publish and resolve working
   ```bash
   ./ipfs-publisher --ipfs-mode embedded --test-ipns
   # IPNS Name: k2k4r8jhoqvl742b4riwpn8uozsroa8bn8nb28myr9uzgr9mfc8x16qg
   # Successfully resolved to CID
   ```

4. ✅ **Repository persistence**: Same Peer ID across runs
   ```bash
   # First run creates repo
   ./ipfs-publisher --ipfs-mode embedded --check-ipfs
   # Peer ID: QmNYH7Z17TCKkwGf45H5qxbRjjbgEmT42EbZM37uasLoYb
   
   # Second run uses existing repo
   ./ipfs-publisher --ipfs-mode embedded --check-ipfs
   # Peer ID: QmNYH7Z17TCKkwGf45H5qxbRjjbgEmT42EbZM37uasLoYb (same)
   ```

5. ✅ **Port checking**: Port availability verified before startup
   ```bash
   # Ports 4002 (swarm), 5002 (API), 8081 (gateway) checked
   ```

6. ✅ **Plugin system**: Datastore plugins (flatfs, levelds, badgerds) loaded correctly
   ```bash
   # No "unknown datastore type" errors
   # Repository created with flatfs datastore
   ```

7. ✅ **Graceful shutdown**: Node stops cleanly
   ```bash
   # SIGINT handled, repository closed properly
   ```

## Phase 4 Test Results (22 Nov 2025)

1. ✅ **PubSub node startup**: Standalone libp2p node created
   ```bash
   ./ipfs-publisher --test-pubsub
   # Peer ID: 12D3KooWQRC9YW6vfEquP89PSnX2ahng5bFXCpqy1t2Uxma1TXfF
   # Listening on: /ip4/127.0.0.1/tcp/50982, /ip4/192.168.100.2/tcp/50982
   ```

2. ✅ **Bootstrap peer connection**: Connected to 5 IPFS bootstrap peers
   ```bash
   # Connected to 5 bootstrap peers
   # Total peers after discovery: 39 peers (0 on topic initially)
   ```

3. ✅ **Keypair generation**: Ed25519 keypair generated successfully
   ```bash
   # Public key: MhvWqUm1qu+Cn7tUP+pmciVEy0bkE6TR...
   ```

4. ✅ **Message creation and signing**: AnnouncementMessage created and signed
   ```bash
   # Version: 1
   # IPNS: k51qzi5uqu5dh9ihj8p0dxgzm4jw8m...
   # Collection Size: 10
   # Timestamp: 1763820505
   # Signature verified
   ```

5. ✅ **Message publishing**: Successfully published to topic
   ```bash
   # Message published to topic: mdn/collections/announce
   ```

6. ✅ **Peer discovery**: DHT peer discovery working
   ```bash
   # Connected to 39 peers after 5 seconds
   # 0 peers on topic (no other publishers yet)
   ```

7. ✅ **Signature verification**: Ed25519 signature validation working
   ```bash
   # Signature verified with public key
   ```

## Next Steps: Phase 5

Phase 5 will implement directory monitoring:
- File system watching with fsnotify
- Automatic file uploads on detection
- NDJSON index management
- State persistence
- Integration with PubSub for automatic announcements

## Development

### Dependencies

- `github.com/spf13/viper` - Configuration management
- `github.com/spf13/pflag` - CLI flags parsing
- `github.com/sirupsen/logrus` - Structured logging
- `gopkg.in/natefinch/lumberjack.v2` - Log rotation
- `github.com/ipfs/go-ipfs-api` - IPFS HTTP API client (external mode)
- `github.com/ipfs/kubo` v0.38.2 - IPFS core implementation (embedded mode)
- `github.com/ipfs/boxo` - IPFS primitives (CID, files, path, etc.)
- `github.com/libp2p/go-libp2p` - P2P networking (PubSub node)
- `github.com/libp2p/go-libp2p-pubsub` v0.15.0 - GossipSub protocol
- `github.com/libp2p/go-libp2p-kad-dht` - DHT for peer discovery

### Technical Notes

#### Kubo v0.38.2 API Changes
When implementing embedded mode, we encountered several API changes in kubo v0.38.2:
- `coreiface` moved from `github.com/ipfs/boxo/coreiface` to `github.com/ipfs/kubo/core/coreiface`
- `Add()` method now requires `files.Node` instead of `io.Reader`
- `Pin` option signature changed to take two parameters: `options.Unixfs.Pin(bool, string)`
- Path parsing now uses `path.NewPath()` from `github.com/ipfs/boxo/path`

#### Plugin System
Embedded mode requires proper datastore plugin initialization:
- Kubo preloads plugins via `plugin/loader/preload.go`
- Import plugins with blank imports: `_ "github.com/ipfs/kubo/plugin/plugins/flatfs"`
- Do NOT manually call `loader.Preload()` - it causes duplicates
- Use `loader.NewPluginLoader("")` to work with preloaded plugins
- Call `Initialize()` and `Inject()` before repository operations

#### Repository Management
- Repository created at configured path (default: `~/.ipfs_publisher/ipfs-repo`)
- Uses flatfs datastore by default
- Persists between runs (same Peer ID)
- Custom ports avoid conflicts with existing IPFS nodes
- Port availability checked before startup

#### PubSub Architecture
PubSub implementation uses a standalone libp2p node (separate from IPFS node):
- **Dual-node design**: IPFS node for content, PubSub node for announcements
- **GossipSub protocol**: Efficient topic-based pub/sub with peer scoring
- **DHT integration**: Uses Kademlia DHT for peer discovery
- **Bootstrap peers**: Connects to default IPFS bootstrap peers for network entry
- **Message format**: JSON with version, IPNS, collection size, timestamp, signature
- **Ed25519 signing**: Messages signed with private key, verified with embedded public key
- **Timestamp validation**: 1-hour drift check prevents replay attacks
- **Topic isolation**: Each application instance can use different topics

#### Message Security
- Ed25519 keypair generation for signing
- Public key embedded in message for verification
- Base64-encoded signatures
- Canonical JSON for consistent signing (sorted keys, no signature field)
- Timestamp-based freshness validation
- No encryption (messages are public announcements)

### Directory Structure

- Application data: `~/.ipfs_publisher/`
- Logs: `~/.ipfs_publisher/logs/app.log`
- Lock file: `~/.ipfs_publisher/.ipfs_publisher.lock`
- Keys (future): `~/.ipfs_publisher/keys/`
- State (future): `~/.ipfs_publisher/state.json`

## License

See main project README for license information.
