# IPFS Media Collection Publisher

A Go application for automatic publishing of media collections to IPFS with announcement via Pubsub.

## Current Status: Phase 2 Complete ✓

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
│   │   └── external.go       # External IPFS HTTP API client
│   ├── logger/
│   │   └── logger.go         # Logging system
│   └── lockfile/
│       └── lockfile.go       # Lock file management
├── config.yaml               # Sample configuration
├── go.mod                    # Go module definition
├── ipfs-publisher           # Compiled binary
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

## Next Steps: Phase 3

Phase 3 will implement embedded IPFS node functionality:
- Repository initialization and management
- Port availability checks
- Node lifecycle management (start/stop)
- Bootstrap peer connection
- Same IPFSClient interface implementation

## Development

### Dependencies

- `github.com/spf13/viper` - Configuration management
- `github.com/spf13/pflag` - CLI flags parsing
- `github.com/sirupsen/logrus` - Structured logging
- `gopkg.in/natefinch/lumberjack.v2` - Log rotation
- `github.com/ipfs/go-ipfs-api` - IPFS HTTP API client
- `github.com/ipfs/boxo` - IPFS primitives (CID, multiaddr, etc.)

### Directory Structure

- Application data: `~/.ipfs_publisher/`
- Logs: `~/.ipfs_publisher/logs/app.log`
- Lock file: `~/.ipfs_publisher/.ipfs_publisher.lock`
- Keys (future): `~/.ipfs_publisher/keys/`
- State (future): `~/.ipfs_publisher/state.json`

## License

See main project README for license information.
