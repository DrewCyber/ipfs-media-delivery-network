# IPFS Media Collection Publisher

A Go application for automatic publishing of media collections to IPFS with announcement via Pubsub. The application monitors directories for media files, uploads them to IPFS, maintains an index, and publishes changes via IPNS and Pubsub.

## Features

### Current (Phase 1 & 2 Complete)

- ✅ **Configuration Management** - YAML-based configuration with validation
- ✅ **IPFS Integration** - External IPFS node support via HTTP API
- ✅ **File Upload** - Upload files to IPFS with configurable options (pin, raw-leaves)
- ✅ **IPNS Support** - Publish and resolve IPNS names
- ✅ **Logging** - Structured logging with file rotation and console output
- ✅ **Lock File** - Prevents multiple instances from running simultaneously
- ✅ **CLI Interface** - Comprehensive command-line interface with multiple flags

### Coming Soon

- 🔄 Embedded IPFS node support
- 🔄 PubSub announcements
- 🔄 Directory monitoring and automatic uploads
- 🔄 NDJSON index management
- 🔄 State persistence and recovery

## Installation

### Prerequisites

- Go 1.21 or higher
- External IPFS node (e.g., IPFS Desktop, kubo daemon) running on default port 5001

### Build from Source

```bash
git clone https://github.com/atregu/ipfs-publisher.git
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

### 3. Check IPFS Connection

```bash
./ipfs-publisher --check-ipfs
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

#### Use Custom Configuration

```bash
./ipfs-publisher --config /path/to/custom/config.yaml
```

#### Override IPFS Mode

```bash
./ipfs-publisher --ipfs-mode external
```

## Configuration

The application uses a YAML configuration file. Here's a complete example:

```yaml
# IPFS node configuration
ipfs:
  # Mode: "external" (use existing IPFS node) or "embedded" (run IPFS inside app)
  mode: "external"
  
  # External node settings (used when mode: external)
  external:
    api_url: "http://localhost:5001"
    timeout: 300  # seconds
    add_options:
      nocopy: false      # Use filestore (requires external node support)
      pin: true          # Pin uploaded files
      chunker: "size-262144"  # Chunking strategy
      raw_leaves: true   # Use raw leaves for UnixFS

  # Embedded node settings (used when mode: embedded) - Coming soon
  embedded:
    repo_path: "~/.ipfs_publisher/ipfs-repo"
    swarm_port: 4002
    api_port: 5002
    gateway_port: 8081

# PubSub configuration - Coming soon
pubsub:
  topic: "mdn/collections/announce"
  announce_interval: 3600  # seconds

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

- **external**: Connects to an existing IPFS node (e.g., IPFS Desktop, kubo daemon)
- **embedded**: Runs a full IPFS node inside the application (coming soon)

#### Add Options

- **pin** (boolean): Pin uploaded files to prevent garbage collection
- **nocopy** (boolean): Use filestore (requires external node with filestore enabled)
- **chunker** (string): Chunking strategy (e.g., "size-262144")
- **raw_leaves** (boolean): Use raw leaves for UnixFS

#### Logging Levels

- **debug**: Detailed information for debugging
- **info**: General informational messages
- **warn**: Warning messages
- **error**: Error messages only

## Testing

### Phase 2 Test Results

All Phase 2 tests pass successfully:

#### Test 1: Check Version
```bash
$ ./ipfs-publisher --version
ipfs-publisher version 0.1.0
```

#### Test 2: Check IPFS Connection
```bash
$ ./ipfs-publisher --check-ipfs
✓ Connected to IPFS node
  Version: 0.38.2
  Node ID: 12D3KooWNZ9Ma5sMmcr3brheC685dgrKJaM9SdhZrHojpKfywjg4
```

#### Test 3: Upload File with Pin
```bash
$ ./ipfs-publisher --test-upload file.mp3
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
$ ./ipfs-publisher --test-ipns
1. Uploading test content to IPFS...
   CID: bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
2. Publishing to IPNS...
   IPNS Name: k51qzi5uqu5dkweh3vfy3ac59oobbnehs3ojsno0sog1nbvc70kt7tgbxvmqgh
   Points to: /ipfs/bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
3. Resolving IPNS name...
   Resolved to: /ipfs/bafkreigawy2oq47r6rvwok3q5u7khmsvfd5r6san657a2k2basbxsiomny
✓ IPNS test successful!
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
│   │   └── external.go          # External IPFS HTTP API client
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
