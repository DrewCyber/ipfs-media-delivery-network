# IPFS Media Collection Publisher

A Go application for automatic publishing of media collections to IPFS with announcement via Pubsub.

## Current Status: Phase 5 Complete ✓

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

**Phase 5: Directory scanning and index creation** ✅
- ✅ Recursive directory scanner with extension filtering
- ✅ Hidden file and temporary file filtering
- ✅ NDJSON index format implementation
- ✅ Index manager with Add/Update/Get operations
- ✅ State manager with JSON persistence
- ✅ File state tracking (CID, mtime, size, indexID)
- ✅ Incremental uploads (skip unchanged files)
- ✅ Progress bar for batch operations (>10 files)
- ✅ --dry-run flag for testing without uploads
- ✅ Index upload to IPFS
- ✅ Version management in state
- ✅ Thread-safe state operations

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
│   ├── scanner/
│   │   └── scanner.go        # Directory scanner with extension filtering
│   ├── index/
│   │   └── manager.go        # NDJSON index manager
│   ├── state/
│   │   └── manager.go        # State persistence and recovery
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

### Scan and Upload Media Collection
```bash
# Dry run - scan without uploading
./ipfs-publisher --dry-run

# Upload all files
./ipfs-publisher
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

## Phase 5 Test Results (22 Nov 2025)

1. ✅ **Directory scanning**: Found 4 files matching criteria
   ```bash
   ./ipfs-publisher --dry-run
   # Found 4 files matching criteria
   # [1] /Users/atregu/test-media/song1.mp3 (15 bytes)
   # [2] /Users/atregu/test-media/song2.mp3 (15 bytes)
   # [3] /Users/atregu/test-media/test.mp3 (5 bytes)
   # [4] /Users/atregu/test-media/video.mkv (11 bytes)
   ```

2. ✅ **File upload to IPFS**: All files uploaded successfully
   ```bash
   ./ipfs-publisher
   # Uploading: song1.mp3
   #    ✓ CID: bafkreicd7xur6y2c7z3vmmprlo5l2cu34azkjfg7myb2sv4polwivxroze
   # Uploading: song2.mp3
   #    ✓ CID: bafkreignvobfvo6srdpdccgidabj7vnoq5m6otjwgvd7mpxsy3ykxhafyq
   # ... (4 files total)
   ```

3. ✅ **NDJSON index creation**: Index file created correctly
   ```bash
   cat ~/.ipfs_publisher/collection.ndjson
   # {"id":1,"CID":"bafkrei...","filename":"song1.mp3","extension":"mp3"}
   # {"id":2,"CID":"bafkrei...","filename":"song2.mp3","extension":"mp3"}
   # {"id":3,"CID":"bafkrei...","filename":"test.mp3","extension":"mp3"}
   # {"id":4,"CID":"bafkrei...","filename":"video.mkv","extension":"mkv"}
   ```

4. ✅ **Index uploaded to IPFS**: Index CID saved in state
   ```bash
   # Index uploaded to IPFS: QmYfa7ERXZH1R3N63GSBVYv1fpMSxZ9J7izgiJE4S6z4pb
   ```

5. ✅ **State persistence**: State saved with version tracking
   ```json
   {
     "version": 1,
     "ipns": "",
     "lastIndexCID": "QmYfa7ERXZH1R3N63GSBVYv1fpMSxZ9J7izgiJE4S6z4pb",
     "files": {
       "/Users/atregu/test-media/song1.mp3": {
         "cid": "bafkrei...",
         "mtime": 1763821114,
         "size": 15,
         "indexId": 1
       }
     }
   }
   ```

6. ✅ **Incremental updates**: Second run skipped unchanged files
   ```bash
   ./ipfs-publisher
   # Loaded state: version=1, files=4
   # Loaded 4 records from index (next ID: 5)
   # Processing complete: 0 uploaded, 4 skipped, 0 errors
   ```

7. ✅ **Extension filtering**: Only configured extensions processed
   ```bash
   # Files with .txt, .jpg, etc. ignored
   # Only .mp3, .mkv, .mp4, .flac, .wav, .avi processed
   ```

8. ✅ **Hidden file filtering**: Hidden files automatically skipped
   ```bash
   # Files starting with . are ignored
   # Temporary files with ~ are ignored
   ```

## Next Steps: Phase 6

Phase 6 will implement IPNS and key management:
- Ed25519 key pair generation
- Key storage with proper permissions (0600)
- IPNS record creation and publishing
- IPNS updates on index changes
- Integration with existing state management

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
- `github.com/schollz/progressbar/v3` v3.18.0 - Progress bar for uploads

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

#### Scanner Architecture
- **Extension filtering**: Case-insensitive map lookup for O(1) performance
- **Hidden file detection**: Files starting with `.` automatically skipped
- **Temporary file detection**: Files with `~` prefix/suffix skipped
- **Recursive traversal**: Uses `filepath.Walk` for directory tree scanning
- **Path expansion**: Tilde (`~`) expanded to home directory
- **Error handling**: Continues scanning on individual file errors

#### Index Format (NDJSON)
- **One JSON object per line**: Enables streaming and append operations
- **Sequential IDs**: Start at 1, increment on new files
- **ID preservation**: IDs never change, even when files deleted
- **Atomic writes**: Use temp file + rename for crash safety
- **Fields**: id (int), CID (string), filename (string), extension (string)
- **No modification time in index**: Stored separately in state.json

#### State Management
- **JSON format**: Human-readable and easy to debug
- **File tracking**: Maps absolute path to FileState (cid, mtime, size, indexId)
- **Version counter**: Increments on each collection change
- **Thread-safe**: Mutex protection for concurrent access
- **Change detection**: Compare mtime and size to detect modifications
- **Atomic writes**: Temp file + rename pattern
- **Recovery**: Load on startup, continue from last state

### Directory Structure

- Application data: `~/.ipfs_publisher/`
- Logs: `~/.ipfs_publisher/logs/app.log`
- Lock file: `~/.ipfs_publisher/.ipfs_publisher.lock`
- Keys (future): `~/.ipfs_publisher/keys/`
- State (future): `~/.ipfs_publisher/state.json`

## License

See main project README for license information.
