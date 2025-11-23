# IPFS Media Collection Publisher

A Go application for automatic publishing of media collections to IPFS with announcement via Pubsub. The application monitors directories for media files, uploads them to IPFS, maintains an index, and publishes changes via IPNS and Pubsub.

## Features

### Current (Phase 1-8 Complete)

- ✅ **Configuration Management** - YAML-based configuration with validation
- ✅ **IPFS Integration** - External IPFS node support via HTTP API
- ✅ **Embedded IPFS Node** - Built-in IPFS node with custom ports and repository
- ✅ **File Upload** - Upload files to IPFS with configurable options (pin, raw-leaves)
- ✅ **IPNS Support** - Publish and resolve IPNS names (works with both modes)
- ✅ **PubSub Announcements** - Mode-aware PubSub support:
  - **Embedded mode**: Uses embedded IPFS node's PubSub (single libp2p instance)
  - **External mode**: Standalone libp2p PubSub node with DHT peer discovery
- ✅ **Message Signing** - Ed25519 signature support for announcements
- ✅ **Directory Scanning** - Recursive scanning with extension filtering
- ✅ **NDJSON Index** - Media collection index with sequential IDs
- ✅ **State Management** - Persistent state with change detection
- ✅ **Real-time Monitoring** - Automatic file change detection with fsnotify
- ✅ **Incremental Updates** - Only process changed/new files
- ✅ **Debouncing** - 300ms debounce for rapid file changes
- ✅ **Automatic Sync** - Index and IPNS automatically updated on file changes
- ✅ **Periodic State Saving** - State persisted every 60 seconds
- ✅ **Progress Bar** - Visual feedback for batch uploads
- ✅ **IPNS Key Management** - Ed25519 keypair generation and secure storage
- ✅ **PubSub Integration** - Announcements after IPNS updates
- ✅ **Periodic Announcements** - Configurable interval (default: 1 hour)
- ✅ **Logging** - Structured logging with file rotation and console output
- ✅ **Lock File** - Prevents multiple instances from running simultaneously
- ✅ **CLI Interface** - Comprehensive command-line interface with multiple flags

### Coming Soon

- 🔄 Web UI for monitoring
- 🔄 Multiple IPNS keys (per-directory)
- 🔄 File metadata (tags, descriptions)

## Installation

### Prerequisites

- Go 1.21 or higher
- **For external mode only**: External IPFS node (e.g., IPFS Desktop, kubo daemon)
  - Default embedded mode requires no external dependencies

### Build from Source

```bash
git clone https://github.com/user/ipfs-publisher.git
cd ipfs-publisher
go build -o ipfs-publisher ./cmd/ipfs-publisher
```

## Quick Start

### 1. Initialize Configuration

```bash
./ipfs-publisher --init
```

This creates a default `config.yaml` file in the current directory.

### 2. Edit Configuration

Edit `config.yaml` to add your media directories:

```yaml
directories:
  - "/path/to/your/media"
  - "/path/to/more/media"

extensions:
  - "mp3"
  - "mp4"
  - "mkv"
  - "avi"
  - "flac"
```

### 3. Choose IPFS Mode

#### Option A: Embedded Mode (default, recommended)

```bash
./ipfs-publisher --check-ipfs
```

Expected output:
```
✓ Embedded IPFS node started successfully. Peer ID: QmXxx...
✓ Listening on 13 addresses
✓ Connected to IPFS node
```

#### Option B: External Mode (for development or existing IPFS Desktop users)

```bash
./ipfs-publisher --ipfs-mode external --check-ipfs
```

Expected output:
```
✓ Connected to IPFS node
  Version: 0.38.2
  Node ID: 12D3KooW...
```

### 4. Test File Upload

```bash
./ipfs-publisher --test-upload /path/to/file.mp3
```

Expected output:
```
✓ Upload successful!
  File: file.mp3
  Size: 1234567 bytes
  CID: QmXxx...
  Pinned: true
```

### 5. Run Publisher

```bash
./ipfs-publisher
```

The application will:
1. Perform initial scan and upload all files
2. Create NDJSON index
3. Publish to IPNS
4. Start real-time monitoring
5. Automatically process new/changed files
6. Keep running indefinitely

**Real-Time Monitoring:**
- Watches all configured directories recursively
- Detects new files, modifications, and deletions
- Debounces rapid changes (300ms delay)
- Updates index and IPNS automatically
- Publishes PubSub announcements
- Saves state every 60 seconds

**File Processing:**
- **New file**: Upload to IPFS, add to index, update IPNS
- **Modified file**: Re-upload, update CID in index, update IPNS  
- **Deleted file**: Remove from index, update IPNS
- **Unchanged file**: Skip (based on mtime and size comparison)

Stop the application with `Ctrl+C` (graceful shutdown).

## Usage

### Command-Line Flags

```
  -c, --config string       Path to config file (default "./config.yaml")
  -v, --version            Show version information
  -h, --help               Show help message
      --init               Initialize configuration and generate keys
      --check-ipfs         Check IPFS connection and exit
      --test-upload FILE   Upload a test file to IPFS and exit
      --test-ipns          Test IPNS publish and resolve
      --dry-run            Scan and show what would be processed without uploading
      --ipfs-mode string   Override IPFS mode from config (external/embedded)
```

### Examples

#### Display Help

```bash
./ipfs-publisher --help
```

#### Display Version

```bash
./ipfs-publisher --version
```

#### Check IPFS Connection

```bash
./ipfs-publisher --check-ipfs
```

Verifies connectivity to your IPFS node and displays version information.

#### Show Peer Information

```bash
./ipfs-publisher --peer-info
```

Displays detailed peer information for your IPFS node and PubSub node:
- **Embedded mode**: Shows IPFS node's peer ID and listen addresses
- **External mode**: Shows both external IPFS peer ID and standalone PubSub node details
- Includes connection commands for subscribing to announcements from other nodes

Example output (external mode):
```
IPFS Node Information:
Mode: external

IPFS Peer ID: 12D3KooWNZ9Ma5sMmcr3brheC685dgrKJaM9SdhZrHojpKfywjg4
API URL: http://localhost:5001

=== Standalone PubSub Node (External Mode) ===
Initializing standalone PubSub node...

PubSub Peer ID: 12D3KooWXYZ123...
Topic: mdn/collections/announce
Connected peers: 8
Topic peers: 2

Listen addresses:
  /ip4/192.168.1.100/tcp/35421/p2p/12D3KooWXYZ123...
  /ip4/127.0.0.1/tcp/35421/p2p/12D3KooWXYZ123...

=== To receive PubSub messages from this node ===
Run this command from your IPFS node:

  ipfs swarm connect /ip4/192.168.1.100/tcp/35421/p2p/12D3KooWXYZ123...

Then subscribe to announcements:
  ipfs pubsub sub mdn/collections/announce
```

#### Upload a Test File

```bash
./ipfs-publisher --test-upload test.mp3
```

Uploads a single file to IPFS to verify your setup is working correctly.

#### Test IPNS Operations

```bash
./ipfs-publisher --test-ipns
```

Tests IPNS publish and resolve functionality by uploading a test file, publishing it to IPNS, and then resolving the IPNS name.

#### Test PubSub Announcements

```bash
./ipfs-publisher --test-pubsub
```

Tests PubSub announcement system by:
- Generating Ed25519 keypair
- Creating standalone libp2p PubSub node
- Connecting to DHT bootstrap peers
- Creating, signing, and verifying announcement message
- Publishing to configured topic

#### Scan and Upload Media Collection

```bash
# Dry run - scan without uploading
./ipfs-publisher --dry-run

# Upload all files to IPFS and create index
./ipfs-publisher
```

Scans configured directories, uploads files to IPFS, creates NDJSON index, and saves state. On subsequent runs, skips unchanged files.

#### Use Custom Configuration

```bash
./ipfs-publisher --config /path/to/custom/config.yaml
```

#### Override IPFS Mode

```bash
# Use external IPFS node
./ipfs-publisher --ipfs-mode external

# Use embedded IPFS node
./ipfs-publisher --ipfs-mode embedded
```

#### Test Embedded IPFS Mode

```bash
# Test file upload with embedded node
./ipfs-publisher --ipfs-mode embedded --test-upload test.mp3

# Test IPNS with embedded node
./ipfs-publisher --ipfs-mode embedded --test-ipns

# Check embedded node status
./ipfs-publisher --ipfs-mode embedded --check-ipfs
```

## Configuration

The application uses a YAML configuration file. Here's a complete example:

```yaml
# IPFS node configuration
ipfs:
  # Mode: "embedded" (run IPFS inside app) or "external" (use existing IPFS node)
  mode: "embedded"  # default
  
  # Embedded node settings (used when mode: embedded)
  embedded:
    repo_path: "~/.ipfs_publisher/ipfs-repo"  # Where to store IPFS data
    swarm_port: 4002      # P2P swarm port (default: 4002)
    api_port: 5002        # API port (default: 5002)
    gateway_port: 8081    # Gateway port (default: 8081)
  
  # External node settings (used when mode: external)
  external:
    api_url: "http://localhost:5001"
    timeout: 300  # seconds
    add_options:
      nocopy: false      # Use filestore (requires external node support)
      pin: true          # Pin uploaded files
      chunker: "size-262144"  # Chunking strategy
      raw_leaves: true   # Use raw leaves for UnixFS

# PubSub configuration
pubsub:
  enabled: true
  topic: "mdn/collections/announce"
  announce_interval: 3600  # seconds (default: 1 hour)
  
  # External mode only: Standalone libp2p node settings
  # (Embedded mode uses IPFS node's PubSub on same port)
  listen_port: 0  # Random port for standalone node (external mode only)
  bootstrap_peers: []  # Optional: custom bootstrap peers (uses IPFS defaults if empty)

# Directories to monitor
directories:
  - "~/media"
  - "/mnt/storage/music"

# File extensions to process (case-insensitive)
extensions:
  - "mp3"
  - "mp4"
  - "mkv"
  - "avi"
  - "flac"
  - "wav"

# Logging configuration
logging:
  level: "info"  # debug, info, warn, error
  file: "~/.ipfs_publisher/logs/app.log"
  max_size: 100  # MB
  max_backups: 5
  console: true  # Also log to console

# Application behavior
behavior:
  scan_interval: 10  # seconds
  batch_size: 10
  progress_bar: true
  state_save_interval: 60  # seconds
```

### Configuration Options

#### IPFS Mode

- **embedded** (default): Runs a full IPFS node inside the application
  - No external IPFS node required - fully standalone
  - Creates its own repository at the configured path
  - Uses custom ports to avoid conflicts with existing IPFS nodes
  - Higher memory footprint but zero external dependencies
  - Recommended for production deployments and most users
  - Good for isolated environments and automatic deployments

- **external**: Connects to an existing IPFS node (e.g., IPFS Desktop, kubo daemon)
  - Requires a running IPFS node on the configured API port (default: 5001)
  - Uses HTTP API to interact with the node
  - Lower memory footprint as IPFS runs in a separate process
  - Good for development or when IPFS Desktop is already running

#### Add Options

- **pin** (boolean): Pin uploaded files to prevent garbage collection
- **nocopy** (boolean): Use filestore (requires external node with filestore enabled)
- **chunker** (string): Chunking strategy (e.g., "size-262144")
- **raw_leaves** (boolean): Use raw leaves for UnixFS

#### PubSub Behavior by Mode

The application uses different PubSub implementations depending on IPFS mode:

**Embedded Mode** (Phase 7):
- Uses the embedded IPFS node's native PubSub capability
- Single libp2p instance (IPFS + PubSub share the same peer ID and ports)
- PubSub peers automatically discovered via IPFS DHT
- No additional network ports required
- More efficient as it leverages existing IPFS connections
- Example: Peer ID `QmVh3Xa5azQXtiyYaUh4VjxcfPEyPFa6PWy6z9MQ1VoW86` on port 4002

**External Mode** (Phase 7.1):
- Creates a standalone lightweight libp2p PubSub node
- Separate libp2p instance (different peer ID from IPFS node)
- Required because IPFS Desktop removed PubSub HTTP API endpoint
- Uses DHT with IPFS bootstrap peers for peer discovery
- Configurable port (default: random) via `pubsub.listen_port`
- Minimal resource overhead (only PubSub, no full IPFS functionality)

**Message Format**:
```json
{
  "version": 1,
  "ipns": "k2k4r8...",
  "publicKey": "CAASogEw...",
  "collectionSize": 42,
  "timestamp": 1700000000,
  "signature": "base64_sig..."
}
```

All messages are signed with Ed25519 for authenticity verification.

#### Logging Levels

- **debug**: Detailed information for debugging
- **info**: General informational messages
- **warn**: Warning messages
- **error**: Error messages only

## Testing

### Phase 2 Test Results (External Mode)

All Phase 2 tests pass successfully with external IPFS mode:

#### Test 1: Check Version
```bash
$ ./ipfs-publisher --version
ipfs-publisher version 0.1.0
```

#### Test 2: Check IPFS Connection
```bash
$ ./ipfs-publisher --ipfs-mode external --check-ipfs
✓ Connected to IPFS node
  Version: 0.38.2
  Node ID: 12D3KooWNZ9Ma5sMmcr3brheC685dgrKJaM9SdhZrHojpKfywjg4
```

#### Test 3: Upload File with Pin
```bash
$ ./ipfs-publisher --ipfs-mode external --test-upload file.mp3
✓ Upload successful!
  CID: bafkreid3cyrzhkewyf6pd4eqb2ughbaxtokpuwi7xeabgxk46yo6qerwya
  Pinned: true

$ ipfs pin ls | grep bafkreid3cyrzhkewyf6pd4eqb2ughbaxtokpuwi7xeabgxk46yo6qerwya
bafkreid3cyrzhkewyf6pd4eqb2ughbaxtokpuwi7xeabgxk46yo6qerwya recursive
```

#### Test 4: Upload File without Pin
```bash
$ ./ipfs-publisher --config config-nopin.yaml --test-upload file.txt
✓ Upload successful!
  CID: bafkreieff4wdvvdsgwxfucfl5bxuinqh4lb25omqiqwe35uxb7xzpahhuy
  Pinned: false

$ ipfs pin ls | grep bafkreieff4wdvvdsgwxfucfl5bxuinqh4lb25omqiqwe35uxb7xzpahhuy
(no output - file is not pinned)
```

#### Test 5: IPNS Operations
```bash
$ ./ipfs-publisher --ipfs-mode external --test-ipns
1. Uploading test content to IPFS...
   CID: bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
2. Publishing to IPNS...
   IPNS Name: k51qzi5uqu5dkweh3vfy3ac59oobbnehs3ojsno0sog1nbvc70kt7tgbxvmqgh
   Points to: /ipfs/bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
3. Resolving IPNS name...
   Resolved to: /ipfs/bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
✓ IPNS test successful!
```

### Phase 3 Test Results (Embedded Mode)

All Phase 3 tests pass successfully with embedded IPFS mode:

#### Test 1: Embedded Node Startup
```bash
$ ./ipfs-publisher --ipfs-mode embedded --check-ipfs
✓ Embedded IPFS node started successfully. Peer ID: QmNYH7Z17TCKkwGf45H5qxbRjjbgEmT42EbZM37uasLoYb
✓ Listening on 13 addresses
✓ Connected to IPFS node
```

#### Test 2: File Upload with Embedded Node
```bash
$ ./ipfs-publisher --ipfs-mode embedded --test-upload test.mp3
✓ Upload successful!
  File: test.mp3
  Size: 33 bytes
  CID: bafkreifddhf4n3f64dknxbpfrp7bbt5luzg643mtmzf5bwde6wmmizwuae
  Pinned: true
```

#### Test 3: IPNS with Embedded Node
```bash
$ ./ipfs-publisher --ipfs-mode embedded --test-ipns
1. Uploading test content to IPFS...
   CID: bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
2. Publishing to IPNS...
   IPNS Name: k2k4r8jhoqvl742b4riwpn8uozsroa8bn8nb28myr9uzgr9mfc8x16qg
   Points to: /ipfs/bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
3. Resolving IPNS name...
   Resolved to: bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
✓ IPNS test successful!
```

#### Test 4: Repository Persistence
```bash
$ ./ipfs-publisher --ipfs-mode embedded --check-ipfs
# First run creates repository
✓ Embedded IPFS node started successfully. Peer ID: QmNYH7Z17TCKkwGf45H5qxbRjjbgEmT42EbZM37uasLoYb

$ ./ipfs-publisher --ipfs-mode embedded --check-ipfs
# Second run uses existing repository (same Peer ID)
✓ Embedded IPFS node started successfully. Peer ID: QmNYH7Z17TCKkwGf45H5qxbRjjbgEmT42EbZM37uasLoYb
```

## Project Structure

```
ipfs-publisher/
├── cmd/
│   └── ipfs-publisher/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── ipfs/
│   │   ├── client.go            # IPFS client interface
│   │   ├── external.go          # External IPFS HTTP API client
│   │   ├── embedded.go          # Embedded IPFS node implementation
│   │   └── repo.go              # Embedded node repository management
│   ├── logger/
│   │   └── logger.go            # Logging system
│   └── lockfile/
│       └── lockfile.go          # Lock file management
├── config.yaml                  # Sample configuration
├── go.mod                       # Go module definition
├── README.md                    # This file
└── IMPLEMENTATION.md            # Implementation details
```

## Application Data

The application stores its data in `~/.ipfs_publisher/`:

```
~/.ipfs_publisher/
├── .ipfs_publisher.lock         # Lock file (prevents multiple instances)
├── logs/
│   └── app.log                  # Application logs (rotated)
├── keys/                        # IPNS keys (coming soon)
│   ├── private.key
│   └── public.key
├── state.json                   # Application state (coming soon)
└── ipfs-repo/                   # Embedded IPFS repo (coming soon, embedded mode only)
```

## Troubleshooting

### IPFS Node Not Available

**Problem**: `IPFS node not available` error

**Solution**: 
1. Make sure your IPFS node is running: `ipfs daemon` or start IPFS Desktop
2. Verify the API URL in your config matches your node: default is `http://localhost:5001`
3. Check IPFS is accessible: `ipfs id`

### Lock File Error

**Problem**: `another instance is already running` error

**Solution**:
1. Check if another instance is running: `ps aux | grep ipfs-publisher`
2. If not, remove stale lock file: `rm ~/.ipfs_publisher/.ipfs_publisher.lock`

### IPNS Publish Timeout (External Mode)

**Problem**: `Failed to publish IPNS: context deadline exceeded` or stuck on "Publishing to IPNS..."

**Cause**: External IPFS node is not responding, hung, or has DHT/networking issues

**Solutions**:
1. **Restart external IPFS daemon**:
   ```bash
   # Stop IPFS
   ipfs shutdown
   # Or kill IPFS Desktop
   
   # Start again
   ipfs daemon
   # Or restart IPFS Desktop
   ```

2. **Check IPFS daemon is actually running**:
   ```bash
   ipfs id
   # Should return peer information immediately
   ```

3. **Verify IPFS API is responding**:
   ```bash
   curl http://localhost:5001/api/v0/version
   # Should return version info quickly
   ```

4. **Check DHT connectivity** (IPNS requires DHT):
   ```bash
   ipfs stats dht
   # Should show routing table with peers
   ```

5. **Use embedded mode instead** (recommended):
   ```bash
   # Edit config.yaml:
   # ipfs:
   #   mode: "embedded"
   
   ./ipfs-publisher --ipfs-mode embedded
   ```
   Embedded mode is more reliable as it doesn't depend on external IPFS daemon health.

6. **Increase timeout** (if external mode is required):
   ```yaml
   # In config.yaml:
   ipfs:
     external:
       timeout: 600  # Increase from 300 to 600 seconds
   ```

7. **Check IPFS daemon logs** for errors:
   ```bash
   # If running ipfs daemon manually, check its terminal output
   # For IPFS Desktop, check: ~/.ipfs/logs/
   ```

**Note**: IPNS publishing can be slow on external IPFS nodes with poor DHT connectivity. Embedded mode provides better control and reliability.

### Files Not Being Pinned

**Problem**: Uploaded files are not pinned

**Solution**:
1. Check your config: `add_options.pin` should be `true`
2. Verify with test upload: `./ipfs-publisher --test-upload file.mp3`
3. The output should show `Pinned: true`

### Permission Denied

**Problem**: Permission errors accessing directories

**Solution**:
1. Check directory permissions: `ls -la /path/to/directory`
2. Ensure the user running ipfs-publisher has read access
3. For media directories, `chmod -R +r /path/to/directory` may help

## Development

### Dependencies

```bash
go get github.com/spf13/viper           # Configuration
go get github.com/spf13/pflag           # CLI flags
go get github.com/sirupsen/logrus       # Logging
go get gopkg.in/natefinch/lumberjack.v2 # Log rotation
go get github.com/ipfs/go-ipfs-api      # IPFS HTTP API
```

### Building

```bash
go build -o ipfs-publisher ./cmd/ipfs-publisher
```

### Running Tests

```bash
go test ./...
```

## Roadmap

### Phase 1: Basic Infrastructure ✅ Complete
- Configuration management
- Logging system
- Lock file mechanism
- CLI interface

### Phase 2: External IPFS Integration ✅ Complete
- HTTP API client
- File upload with options
- IPNS operations
- Connection testing

### Phase 3: Embedded IPFS Node (In Progress)
- Repository initialization
- Node lifecycle management
- Port configuration
- Bootstrap peers

### Phase 4: PubSub Announcements
- Embedded libp2p PubSub node
- Message signing
- Periodic announcements
- Message format v3

### Phase 5: Directory Monitoring
- File system watching
- Change detection
- NDJSON index creation
- Incremental updates

### Phase 6: State Management
- State persistence
- Recovery after restart
- Index management
- Version tracking

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[License details to be added]

## Support

For issues and questions:
- GitHub Issues: [https://github.com/atregu/ipfs-publisher/issues](https://github.com/atregu/ipfs-publisher/issues)
- Documentation: See IMPLEMENTATION.md for detailed implementation notes

## Acknowledgments

- Built with [kubo](https://github.com/ipfs/kubo) IPFS implementation
- Uses [go-ipfs-api](https://github.com/ipfs/go-ipfs-api) for HTTP API communication
- Structured logging with [logrus](https://github.com/sirupsen/logrus)
