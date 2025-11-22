# Product Requirements Document: IPFS Media Collection Publisher

## 1. Overview

### 1.1 Purpose
Go application for automatic publishing of media collections to IPFS with announcement via Pubsub. The application watches configured directories, uploads media files to IPFS, creates an index file with metadata and publishes a pointer to the collection via IPNS and Pubsub.

### 1.2 Target Users
- Users who want to distribute media collections over IPFS
- Hosts of decentralized media libraries
- Content providers in P2P networks

## 2. Functional Requirements

### 2.1 File System Monitoring

#### 2.1.1 Directory watching
- The application watches a list of directories specified in configuration
- Recursive scanning of subdirectories
- Use an OS-level file system watcher (`fsnotify` library for Go)
- Track events:
  - New file creation
  - Modification of existing file (modification time)
  - File deletion
  - File rename

#### 2.1.2 File filtering
- Process only files with extensions from the whitelist in config
- Case-insensitive extension comparison
- Ignore hidden files (starting with `.`)
- Ignore temporary files and system directories

#### 2.1.3 Change detection
- When a file change is detected, verify by:
  - Modification time (mtime)
  - File size
- Hash comparison is not used for performance optimization

### 2.2 IPFS Integration

#### 2.2.1 IPFS Node Modes

The application supports two modes of IPFS node operation:

**Embedded Mode (default):**
- Launch a full-featured IPFS node inside the application
- Complete IPFS functionality including DHT, bitswap, and content routing
- PubSub uses the same libp2p instance as IPFS node
- Zero external dependencies - fully standalone
- Recommended for most users and production deployments

**External Mode:**
- Connect to an existing IPFS node (e.g., IPFS Desktop, kubo daemon)
- Use HTTP API for file operations (add, pin, get)
- Separate lightweight libp2p node for PubSub (because external IPFS nodes don't provide PubSub API)
- Useful for development or when IPFS Desktop is already running

**PubSub Architecture:**
- **Embedded IPFS mode**: PubSub runs on the same libp2p instance as IPFS
  - Single peer identity
  - Single swarm port
  - PubSub automatically enabled during IPFS node initialization
- **External IPFS mode**: Separate lightweight libp2p node for PubSub only
  - Own peer identity (different from external IPFS node)
  - Own swarm port
  - Uses same bootstrap peers as IPFS for network connectivity
  - DHT enabled for peer discovery

**Mode Selection:**
- Hardcoded in configuration file
- User explicitly chooses mode
- No automatic fallback between modes
- Mode change requires application restart

#### 2.2.2 Connecting to External IPFS
- Connect to an IPFS node through the HTTP API
- Connection parameters from config (URL, port)
- If the node is unavailable:
  - Log an ERROR
  - Retry connection every 30 seconds
  - Application does not exit, waits for node availability

#### 2.2.3 Embedded IPFS Node

**Initialization:**
- On first run initialize IPFS repository at `~/.ipfs_publisher/ipfs-repo`
- Generate peer identity (keypair)
- Use standard IPFS bootstrap nodes for DHT connectivity
- Repository persists between runs

**Configuration:**
- Custom ports to avoid conflicts with external nodes
- Configurable swarm, API, and gateway ports
- Default ports different from standard IPFS (4001, 5001, 8080)

**Startup Checks:**
- Before starting, check if configured ports are available
- If any port is occupied:
  - Log ERROR: "Port {port} is already in use. Please check if another IPFS node is running or change ports in config."
  - Exit application with non-zero status code
  - Suggest checking `ipfs id` or `lsof -i :{port}`

**Lifecycle:**
- Start embedded node on application startup
- Graceful shutdown when application stops
- Wait for pending operations before shutdown

**Features:**
- Full IPFS node capabilities
- DHT participation for content routing
- Bitswap for content exchange
- Content pinning and storage
- Garbage collection (optional, configurable)

#### 2.2.4 Uploading files to IPFS
- Upload files sequentially (one at a time)
- Use active IPFS node (external or embedded)
- Support IPFS add options:
  - `--nocopy` (optional, from config, only for external mode with filestore)
  - `--pin` (optional, from config)
  - `--chunker` (optional, from config)
  - `--raw-leaves` (optional, from config)
  - Other options via config
- Obtain CID for each uploaded file
- Log the process:
  - INFO: start uploading file
  - INFO: successful upload with CID
  - ERROR: upload error with details

#### 2.2.5 Progress Tracking
- Show a progress bar when processing a large number of files (>10)
- Progress bar info:
  - Current file being processed
  - Processed/total count
  - Percent complete
  - Current IPFS mode indicator
- Detailed logs are written to a log file in parallel with the progress bar

### 2.3 Collection Index Management

#### 2.3.1 NDJSON file format
```json
{"id":1,"CID":"Qmd7EioyCPrbGMTry4XSXL82jnBNcUSTN5hkiVv96Pipxx","filename":"song.mp3","extension":"mp3"}
{"id":2,"CID":"Qmd7EioyCPrbGMTry4XSXL82jnBNcsdfasdfadfasdfasd","filename":"video.mkv","extension":"mkv"}
```

**Fields:**
- `id` (int): Sequential record number (starts from 1)
- `CID` (string): IPFS CID of the uploaded file
- `filename` (string): Original filename
- `extension` (string): File extension (without the dot)

#### 2.3.2 Creating and updating the index
- On first run create an empty NDJSON file
- When adding a new file:
  - Append a new record to the NDJSON
  - Assign `id` the next sequential number
- When an existing file changes:
  - Look up the record by `filename`
  - Update only the `CID` field
  - `id` remains unchanged
- When a file is deleted:
  - Remove the corresponding line from the NDJSON
  - `id` of other records DO NOT change (gaps in numbering are allowed)
- When a file is renamed:
  - Update the `filename` field in the existing record
  - Preserve `id` and `CID`

#### 2.3.3 Uploading the index to IPFS
- After updating the NDJSON, upload the index file to IPFS
- Use active IPFS node (external or embedded)
- Obtain the CID for the index file
- Pin the index file (if enabled in config)

### 2.4 IPNS Management

#### 2.4.1 Key generation
- On first run generate an Ed25519 key pair
- Keys are stored locally in:
  - `~/.ipfs_publisher/keys/private.key`
  - `~/.ipfs_publisher/keys/public.key`
- On subsequent runs the existing keys are used
- One IPNS key for the entire application (all directories)

#### 2.4.2 Creating and updating IPNS
- On the first publish create an IPNS record
- The IPNS record points to the current CID of the index file
- Use active IPFS node (external or embedded) for IPNS operations
- On index updates:
  - Update the IPNS record to the new CID
  - The IPNS hash remains the same
- IPNS record TTL: 24 hours
- IPNS record is signed with the private key

### 2.5 Pubsub Announcement

#### 2.5.1 PubSub Architecture

**Embedded IPFS Mode:**
- PubSub runs on the same libp2p/IPFS node instance
- No separate PubSub node needed
- Uses IPFS node's peer identity and swarm port
- PubSub protocol automatically enabled during IPFS node initialization
- Participates in IPFS DHT for peer discovery

**External IPFS Mode:**
- Separate lightweight libp2p node for PubSub only
- Reason: External IPFS nodes (IPFS Desktop, kubo) removed PubSub endpoint from HTTP API
- Own peer identity (independent from external IPFS node)
- Own swarm port (configurable)
- Uses same bootstrap peers as IPFS for network connectivity
- DHT enabled for announcing and discovering peers subscribed to topics
- Minimal resource footprint (only PubSub and DHT protocols)

#### 2.5.2 Message format
```json
{
  "version": 3,
  "ipns": "k51qzi5uqu5dh9ihj8p0dxgzm4jw8m8q9tqxm...",
  "publicKey": "CAASogEwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC...",
  "collectionSize": 60,
  "timestamp": 1700000000,
  "signature": "base64_encoded_signature"
}
```

**Fields:**
- `version` (int): Update counter (incremented on each collection change)
- `ipns` (string): IPNS hash pointing to the current index version
- `publicKey` (string): Base64-encoded public key for verification
- `collectionSize` (int): Number of records in the NDJSON file
- `timestamp` (int): Unix timestamp in seconds
- `signature` (string): Base64-encoded signature of the message

#### 2.5.3 Signing messages
- The signature is created over the JSON object without the `signature` field
- Algorithm: Ed25519
- Signature is Base64 encoded
- Receivers can verify the signature using `publicKey`

#### 2.5.4 Publishing to Pubsub
- Topic: configurable in config (default `mdn/collections/announce`)
- Published via:
  - **Embedded mode**: IPFS node's libp2p instance
  - **External mode**: Standalone PubSub libp2p node
- Publish occurs:
  - On the first upload of all files
  - After each collection update
  - Every hour (regardless of changes)
- On publish error:
  - Log an ERROR
  - Wait for the next attempt (after an hour or on next change)
  - Application continues running

#### 2.5.5 Periodic announcements
- Timer: every 60 minutes
- Publish the current state of the collection
- `version` is not incremented if the collection did not change
- `timestamp` remains unchanged (time of the last real collection update)

### 2.6 State Persistence

#### 2.6.1 Local state
State file: `~/.ipfs_publisher/state.json`

```json
{
  "version": 15,
  "ipns": "k51qzi5uqu5dh9ihj8p0dxgzm4jw8m8q9tqxm...",
  "lastIndexCID": "QmXyZ...",
  "files": {
    "/path/to/file1.mp3": {
      "cid": "QmAbc...",
      "mtime": 1700000000,
      "size": 5242880,
      "indexId": 1
    },
    "/path/to/file2.mkv": {
      "cid": "QmDef...",
      "mtime": 1700000100,
      "size": 104857600,
      "indexId": 2
    }
  }
}
```

#### 2.6.2 Recovery after restart
- On startup the application loads `state.json`
- Scan configured directories to determine changes:
  - New files (absent from state)
  - Modified files (mtime or size differs)
  - Deleted files (present in state but missing on disk)
- Process only changed files
- Update state after processing

#### 2.6.3 Handling interrupted uploads
- If a file upload was interrupted due to application crash:
  - The file may be missing from state or have an empty CID
  - On restart the file is marked for reprocessing
- Check index integrity on startup

### 2.7 Error Handling

#### 2.7.1 IPFS unavailable (External mode)
- Wait with periodic retry attempts (30s)
- File processing queue accumulates
- Process queue after connection is restored

#### 2.7.2 Embedded IPFS startup failure
- If embedded node fails to start:
  - Log detailed error (port conflict, repo corruption, etc.)
  - For port conflicts: suggest port configuration
  - For repo issues: suggest repo cleanup or migration
  - Exit application with error code

#### 2.7.3 File deleted during processing
- Catch "file not found" errors when reading/uploading
- Remove file from processing queue
- Remove record from NDJSON if it existed
- Update index and publish changes

#### 2.7.4 Insufficient disk space
- Check available disk space before processing large files
- For embedded mode: monitor repo size
- If insufficient:
  - Log ERROR with warning
  - Skip the file
  - Continue with other files

#### 2.7.5 Incorrect permissions
- Catch "permission denied" errors
- Log with the problematic file
- Skip the file and continue

## 3. Configuration

### 3.1 Configuration file
Format: YAML
Default path: `./config.yaml` or `~/.ipfs_publisher/config.yaml`

```yaml
# IPFS node configuration
ipfs:
  # Mode: "embedded" (run IPFS inside app) or "external" (use existing IPFS node)
  mode: "embedded"  # default
  
  # External node settings (used when mode: external)
  external:
    api_url: "http://localhost:5001"
    timeout: 300  # seconds
    add_options:
      nocopy: false  # only works with filestore enabled on external node
      pin: true
      chunker: "size-262144"
      raw_leaves: true
  
  # Embedded node settings (used when mode: embedded)
  embedded:
    repo_path: "~/.ipfs_publisher/ipfs-repo"
    
    # Network ports (must be different from external IPFS node if both run)
    swarm_port: 4002      # libp2p swarm (default IPFS: 4001)
    api_port: 5002        # HTTP API (default IPFS: 5001)
    gateway_port: 8081    # HTTP gateway (default IPFS: 8080)
    
    # Storage settings
    add_options:
      pin: true
      chunker: "size-262144"
      raw_leaves: true
    
    # Bootstrap peers (leave empty to use standard IPFS bootstrap nodes)
    bootstrap_peers: []
    
    # Garbage collection
    gc:
      enabled: true
      interval: 86400  # seconds (24 hours)
      min_free_space: 1073741824  # bytes (1GB)

# PubSub configuration
pubsub:
  topic: "mdn/collections/announce"
  announce_interval: 3600  # seconds (1 hour)
  
  # Standalone PubSub node settings (used only in external IPFS mode)
  # In embedded IPFS mode, PubSub uses the same libp2p instance as IPFS
  standalone:
    bootstrap_peers: []  # leave empty for standard IPFS bootstrap nodes
    swarm_port: 4003     # libp2p swarm port for standalone PubSub node
    enable_dht: true     # DHT for peer discovery and topic announcement

# Directories to monitor
directories:
  - "/path/to/media1"
  - "/path/to/media2"
  - "/home/user/music"

# File extensions to process (case-insensitive)
extensions:
  - "mp3"
  - "mp4"
  - "mkv"
  - "avi"
  - "flac"
  - "wav"

# Logging
logging:
  level: "info"  # debug, info, warn, error
  file: "~/.ipfs_publisher/logs/app.log"
  max_size: 100  # MB
  max_backups: 5
  console: true

# Application behavior
behavior:
  scan_interval: 10  # seconds, how often to check for changes
  batch_size: 10  # files to process in one batch
  progress_bar: true
  state_save_interval: 60  # seconds
```

### 3.2 Command Line Arguments
```bash
ipfs-publisher [flags]

Flags:
  -c, --config string     Path to config file (default "./config.yaml")
  -v, --version          Show version information
  -h, --help             Show help message
  --init                 Initialize configuration and generate keys
  --check-ipfs           Check IPFS connection and exit
  --dry-run              Scan and show what would be processed without uploading
  --ipfs-mode string     Override IPFS mode from config (external/embedded)
```

## 4. Technical Architecture

### 4.1 Technology Stack
- **Language**: Go 1.21+
- **IPFS Integration**: 
  - External mode: `github.com/ipfs/go-ipfs-api` (HTTP API client)
  - Embedded mode: `github.com/ipfs/kubo` (full IPFS node)
- **PubSub**: `github.com/libp2p/go-libp2p-pubsub` (embedded in IPFS or standalone)
- **File System Monitoring**: `github.com/fsnotify/fsnotify`
- **Logging**: `github.com/sirupsen/logrus` or `go.uber.org/zap`
- **Configuration**: `github.com/spf13/viper`
- **Progress Bar**: `github.com/schollz/progressbar`
- **Cryptography**: `crypto/ed25519` (standard library)

### 4.2 Application Components

#### 4.2.1 Main Components
1. **FileWatcher**: Filesystem monitoring
2. **IPFSClient**: Interaction with IPFS (external or embedded)
3. **EmbeddedIPFS**: Embedded IPFS node lifecycle management
4. **StandalonePubSub**: Standalone libp2p PubSub node (only for external IPFS mode)
5. **IndexManager**: NDJSON index management
6. **IPNSManager**: Create and update IPNS records
7. **PubsubPublisher**: Publish announcements to Pubsub (mode-aware)
8. **StateManager**: Save and restore state
9. **KeyManager**: Key generation and management

#### 4.2.2 Data Flow
```
FileWatcher → IPFSClient → IndexManager → IPNSManager
                ↓              ↓             ↓
         StateManager    PubsubPublisher (mode-aware)
                              ↓
                    ┌─────────┴─────────┐
                    │                   │
            Embedded IPFS         Standalone PubSub
            (with PubSub)         (external mode only)
```

#### 4.2.3 IPFS Mode Architecture

**External Mode:**
```
Application
├── HTTP API Client → External IPFS Node
│   ├── Add files
│   ├── Pin content
│   └── IPNS publish
└── Standalone PubSub Node (libp2p)
    ├── Own peer identity
    ├── Own swarm port (4003)
    ├── DHT for peer discovery
    ├── Same bootstrap as IPFS
    └── Announce messages
```

**Embedded Mode:**
```
Application
└── Embedded IPFS Node (full libp2p instance)
    ├── Add files
    ├── Pin content
    ├── IPNS publish
    ├── DHT routing
    ├── Bitswap
    └── PubSub (on same libp2p instance)
        └── Announce messages
```

### 4.3 Concurrency Model
- Main goroutine for FileWatcher
- Separate goroutine for periodic Pubsub announcements
- Separate goroutine for embedded IPFS node (if used in embedded mode)
- Separate goroutine for standalone PubSub node (if used in external mode)
- Worker pool (optional) for parallel uploads to IPFS
- Channels for coordination between components
- Mutex to protect shared state

### 4.4 Performance Considerations

#### 4.4.1 Optimization for large collections
- Incremental index updates (no full re-scan)
- Cache CIDs for unchanged files
- Batch processing for multiple changes
- Debouncing for frequent changes (300ms)

#### 4.4.2 Memory Management
- Streaming upload for large files (>100MB)
- Limit buffer size when reading the index
- Periodic cache eviction for state
- Monitor embedded IPFS repo size

#### 4.4.3 Embedded Mode Considerations
- Repository cleanup with garbage collection
- Configurable storage limits
- Monitor peer connections and DHT performance

## 5. Security Considerations

### 5.1 Key Management
- Private keys stored with permissions `0600`
- Keys directory permissions `0700`
- Optional encryption of keys with a passphrase (future enhancement)

### 5.2 Input Validation
- Validate directory paths (protect from path traversal)
- Check maximum file size
- Validate file extensions
- Sanitize filenames in the index

### 5.3 IPFS Security
- Optional authentication to the IPFS API (external mode)
- Verify TLS certificates when using HTTPS
- Rate limiting to protect from DoS

### 5.4 Embedded Node Security
- Isolated network namespace (optional)
- Firewall rules for swarm port
- Peer filtering (optional)

## 6. Monitoring and Observability

### 6.1 Logging Levels
- **DEBUG**: Detailed info about every operation
- **INFO**: Major events (file processed, index updated, node started)
- **WARN**: Potential problems (processing slowdowns, port conflicts)
- **ERROR**: Errors that do not stop the application

### 6.2 Metrics (optional)
- Number of processed files
- Total size uploaded
- Time to process files
- Number of errors by type
- IPFS node availability
- Embedded node stats (peers, bandwidth)
- PubSub message delivery rate

### 6.3 Health Checks
- Check IPFS connectivity (mode-aware)
- Check embedded node health (if applicable)
- Check PubSub node connectivity
- Check directory availability
- Check integrity of the state file
- Optional status endpoint

## 7. Testing Strategy

### 7.1 Unit Tests
- Tests for each component in isolation
- Mock IPFS API for testing IPFSClient
- Tests for correct signing/verification
- Tests for embedded node lifecycle

### 7.2 Integration Tests
- Tests with external IPFS node
- Tests with embedded IPFS node
- Tests for PubSub message delivery
- Tests for file change scenarios
- Tests for recovery after failures
- Tests for mode switching

### 7.3 Performance Tests
- Tests with large collections (10000+ files)
- Tests with large files (>1GB)
- Memory leak tests for long-running operation
- Embedded node resource usage tests

## 8. Future Enhancements

### 8.1 Potential Features
- Support multiple IPNS keys (per-directory)
- Web UI for monitoring
- Remote directories (SFTP, S3)
- Automatic cleaning of old versions in IPFS
- File metadata (tags, descriptions)
- Playlists and albums support
- Hybrid mode (external for storage, embedded for PubSub)

### 8.2 Optimization Opportunities
- Parallel file uploads to IPFS
- Deduplication by content hash
- Compression for the index file
- Incremental IPNS updates
- Advanced peer routing strategies

## 9. Edge Cases and Limitations

### 9.1 Known Limitations
- Maximum index file size: ~100MB (IPFS block size constraints)
- Remote file versioning not supported
- No automatic rotation of IPNS keys
- No built-in replication to other IPFS nodes
- Embedded node requires more resources than external mode

### 9.2 Edge Cases
- **Rapid multiple changes**: Debounce 300ms
- **Cyclic symlinks**: Ignored during scan
- **Very long filenames**: Truncate to 255 characters
- **Special characters in filenames**: URL-encode in index
- **Duplicate filenames in different directories**: Add relative path to `filename`
- **Port conflicts**: Application exits with error message
- **Repository corruption**: Suggest repo cleanup in error message

## 10. Acceptance Criteria

### 10.1 Functional
- ✓ Application correctly handles adding new files
- ✓ File modification updates its CID in the index
- ✓ File deletion removes it from the index
- ✓ IPNS updates correctly on changes
- ✓ Pubsub messages are published every hour
- ✓ Application recovers state after restart
- ✓ Both external and embedded IPFS modes work
- ✓ PubSub works in both IPFS modes

### 10.2 Non-Functional
- ✓ Processing 1000 files takes < 5 minutes (with `--nocopy` in external mode)
- ✓ Memory usage < 500MB with 10000 files (external mode)
- ✓ Memory usage < 1GB with 10000 files (embedded mode)
- ✓ Application recovers from IPFS unavailability in < 1 minute
- ✓ 99.9% uptime during continuous 30-day operation
- ✓ Embedded node starts within 30 seconds
- ✓ Port conflict detected and reported before node start

## 11. Risks and Mitigations

### 11.1 Critical Risks
1. **Race condition during rapid file changes**
   - Problem: File may change during upload to IPFS
   - Mitigation: Verify mtime after upload, reprocess if mismatch

2. **Data loss between index update and state write**
   - Problem: State not written atomically with IPFS operations
   - Mitigation: Not critical for MVP. On restart the app will rescan directories and reconcile differences

3. **Conflicts when multiple instances run concurrently**
   - Problem: Two instances may process the same files
   - Mitigation: Lock file (`~/.ipfs_publisher/.lock`) with PID check to prevent multiple runs

4. **Embedded node port conflicts**
   - Problem: Cannot start if ports are occupied
   - Mitigation: Pre-startup port availability check with clear error messages

5. **Embedded node repository corruption**
   - Problem: Crashes or improper shutdown may corrupt repo
   - Mitigation: Proper shutdown handlers, repo lock files, recovery procedures

### 11.2 Scaling Issues
1. **NDJSON index grows without bound**
   - Problem: With tens of thousands of files the index becomes huge (>100MB)
   - Risk: IPFS block size limit, slow index upload/processing
   - Mitigation: Monitor index size. If exceeding threshold (e.g., 50MB) consider splitting indexes or moving to a different format (e.g., SQLite in IPFS)

2. **Memory spike when loading large index**
   - Problem: Entire NDJSON loaded into memory for updates
   - Mitigation: Streamed parsing and line-by-line index updates

3. **Version counter overflow**
   - Problem: With many updates an int may overflow
   - Mitigation: Use `uint64` (sufficient for ~1.8e19 updates)

4. **Embedded node storage growth**
   - Problem: Repository grows indefinitely
   - Mitigation: Configurable garbage collection, storage limits

### 11.3 Reliability Issues
1. **IPNS republish may not keep up with changes**
   - Problem: IPNS TTL 24 hours may cause staleness between updates
   - Mitigation: Periodic IPNS refresh (every 12 hours) even without changes

2. **Pubsub messages are not guaranteed delivered**
   - Problem: Receivers may miss announcements
   - Mitigation: Hourly repeats mitigate missed messages

3. **No verification of successful IPFS persistency**
   - Problem: IPFS API returns CID but data might be corrupted or incomplete
   - Mitigation: Trust IPFS API. Optional verification by re-reading file from IPFS can be added (future)

4. **Standalone PubSub node isolation**
   - Problem: Separate PubSub node (in external mode) may have peer discovery issues
   - Mitigation: Use standard IPFS bootstrap peers, enable DHT, monitor peer count

### 11.4 UX Issues
1. **No progress indication on first run**
   - Problem: User cannot see progress
   - Mitigation: Detailed progress bar with ETA

2. **Unclear why a file is not processed**
   - Problem: File can be ignored for many reasons (extension, permissions)
   - Mitigation: Explicit WARNING logs for ignored files

3. **Embedded mode complexity**
   - Problem: Users may not understand difference between modes
   - Mitigation: Clear documentation, sensible defaults, mode indicator in logs

### 11.5 Security Issues
1. **Public key in Pubsub can be spoofed**
   - Problem: Attacker can publish a different `publicKey`
   - Mitigation: Receivers should maintain a whitelist of trusted keys

2. **No spam protection in Pubsub**
   - Problem: Attacker can flood the topic
   - Mitigation: Rate limiting on receivers

3. **Embedded node network exposure**
   - Problem: Swarm port exposed to internet
   - Mitigation: Firewall configuration guidance, optional private networks

### 11.6 Operational Risks
1. **No automatic cleanup of old CIDs in IPFS**
   - Problem: IPFS node may run out of space with frequent updates
   - Mitigation: Periodic garbage collection or unpinning old versions (especially for embedded mode)

2. **Debugging difficulty for IPFS issues**
   - Problem: IPFS API errors may be uninformative
   - Mitigation: Verbose logging of IPFS requests/responses, embedded node debug logs

3. **Port management complexity**
   - Problem: Users may not know which ports to configure
   - Mitigation: Sensible defaults, clear error messages, documentation

## 12. Remediation Recommendations

### 12.1 High Priority (MVP)
1. Add a lock file to prevent multiple runs
2. Implement integrity check (mtime after upload)
3. Use `uint64` for the version counter
4. Implement graceful shutdown with state save
5. Port availability check before embedded node start
6. Separate PubSub node with proper bootstrap

### 12.2 Medium Priority (v1.1)
1. Implement streaming index processing for large collections
2. Add index size monitoring with alerts
3. Add retry logic with exponential backoff for IPFS operations
4. Implement periodic IPNS refresh (every 12 hours)
5. Embedded node garbage collection
6. PubSub peer count monitoring (mode-aware)
7. Optimize standalone PubSub node resource usage

### 12.3 Low Priority (future)
1. Optional verification of uploaded files (config flag)
2. Chunked indexes for collections >100k files
3. Add metrics and monitoring
4. Implement automatic garbage collection of old CIDs
5. Add a web UI for monitoring
6. Hybrid IPFS mode support

## 13. Implementation Plan (Phased)

### Phase 1: Basic structure and configuration (1-2 days)
**Goal**: Set up project, configuration and basic components

**Tasks:**
1. Initialize Go module and project structure
2. Implement YAML configuration loading with IPFS mode selection
3. Set up logging (file + console)
4. Implement lock file mechanism
5. Basic CLI skeleton with flags
6. Configuration validation (port conflicts, path checks)

**Project structure:**
```
ipfs-publisher/
├── cmd/
│   └── ipfs-publisher/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── logger/
│   │   └── logger.go
│   └── lockfile/
│       └── lockfile.go
├── config.yaml
└── go.mod
```

**Manual tests:**
```bash
# Test 1: Run with config
./ipfs-publisher --config ./config.yaml

# Test 2: Lock file check
./ipfs-publisher  # first instance
./ipfs-publisher  # second instance should exit with an error

# Test 3: Logging
cat ~/.ipfs_publisher/logs/app.log

# Test 4: Config validation
./ipfs-publisher --config invalid.yaml  # should error

# Test 5: IPFS mode validation
# Set invalid mode in config, expect error
```

**Readiness criteria:**
- ✓ Application starts and reads config
- ✓ IPFS mode configuration parsed correctly
- ✓ Logs written to file and console
- ✓ Second instance cannot start
- ✓ Graceful shutdown on Ctrl+C

---

### Phase 2: External IPFS client and basic operations (2-3 days)
**Goal**: Connect to external IPFS and implement file upload

**Tasks:**
1. Implement `IPFSClient` interface
2. Implement external IPFS client (HTTP API)
3. Implement connection to external IPFS API
4. Implement function to add a file to IPFS
5. Error handling and retry logic
6. Support `--nocopy` and `--pin` options
7. IPNS operations via external node

**New files:**
```
internal/
├── ipfs/
│   ├── client.go        # Interface
│   ├── external.go      # HTTP API implementation
│   └── options.go
```

**Manual tests:**
```bash
# Prepare: start external IPFS daemon
ipfs daemon

# Test 1: Check connection
./ipfs-publisher --check-ipfs --ipfs-mode external
# Expect: "✓ Connected to IPFS node at http://localhost:5001"

# Test 2: Upload a single file (test command)
./ipfs-publisher --test-upload /path/to/test.mp3
# Expect: "Uploaded: test.mp3 -> QmXxx..."

# Test 3: --nocopy mode
./ipfs-publisher --test-upload /path/to/test.mp3 --nocopy
# Expect: successful upload with nocopy flag

# Test 4: IPFS unavailable handling
# Stop ipfs daemon
./ipfs-publisher --check-ipfs
# Expect: retries every 30s

# Test 5: Pinning
./ipfs-publisher --test-upload /path/to/test.mp3
ipfs pin ls | grep QmXxx
# Expect: file is pinned

# Test 6: IPNS operations
./ipfs-publisher --test-ipns
# Expect: IPNS record created and resolvable
```

**Readiness criteria:**
- ✓ Successful connection to external IPFS API
- ✓ Files upload and return CIDs
- ✓ `--nocopy` mode works
- ✓ Pinning works
- ✓ Application waits for IPFS when unavailable
- ✓ IPNS publish/resolve works

---

### Phase 3: Embedded IPFS node (3-4 days)
**Goal**: Implement full embedded IPFS node functionality

**Tasks:**
1. Implement embedded IPFS node wrapper
2. Repository initialization and management
3. Port availability checks
4. Node lifecycle management (start/stop)
5. Configuration of swarm/API/gateway ports
6. Bootstrap peer connection
7. Graceful shutdown with cleanup
8. Implement same IPFSClient interface for embedded node

**New files:**
```
internal/
├── ipfs/
│   ├── embedded.go      # Embedded node implementation
│   └── repo.go          # Repository management
```

**Manual tests:**
```bash
# Test 1: First run with embedded mode
rm -rf ~/.ipfs_publisher/ipfs-repo
./ipfs-publisher --ipfs-mode embedded
# Expect: 
# - Repository initialization
# - Node startup logs
# - Peer connections established

# Test 2: Port conflict detection
# Start external IPFS on default ports
ipfs daemon &
# Try to start with conflicting ports in config
./ipfs-publisher --ipfs-mode embedded
# Expect: Error message about port 4002/5002/8081 being occupied

# Test 3: Custom ports
# Update config with custom ports
./ipfs-publisher --ipfs-mode embedded
netstat -tuln | grep <custom_port>
# Expect: ports listening

# Test 4: Bootstrap and peer discovery
./ipfs-publisher --ipfs-mode embedded
# Wait 60 seconds
# Check logs for peer connections
# Expect: "Connected to X peers"

# Test 5: Repository persistence
./ipfs-publisher --ipfs-mode embedded
# Upload some files
# Stop and restart
./ipfs-publisher --ipfs-mode embedded
# Expect: Repository reused, peer identity preserved

# Test 6: Graceful shutdown
./ipfs-publisher --ipfs-mode embedded
# Send SIGTERM or Ctrl+C
# Expect: "Shutting down embedded IPFS node..." then clean exit

# Test 7: Upload via embedded node
./ipfs-publisher --ipfs-mode embedded --test-upload /path/to/test.mp3
# Expect: File added successfully
# Verify from external node:
ipfs cat <CID>
```

**Readiness criteria:**
- ✓ Embedded node initializes repository
- ✓ Port conflicts detected before startup
- ✓ Node starts and accepts connections
- ✓ Bootstrap peers connected
- ✓ Files can be added via embedded node
- ✓ Repository persists between runs
- ✓ Graceful shutdown works
- ✓ Same IPFSClient interface as external mode

---

### Phase 4: Embedded PubSub node (2-3 days)
**Goal**: Implement PubSub on embedded IPFS node for embedded mode

**Tasks:**
1. Enable PubSub protocol on embedded IPFS node
2. Implement GossipSub configuration
3. Message publishing via IPFS node's libp2p instance
4. Integration with PubsubPublisher
5. Signature generation and verification

**New files:**
```
internal/
├── pubsub/
│   ├── embedded.go      # PubSub via embedded IPFS
│   ├── publisher.go     # Message publisher
│   └── message.go       # Message format and signing
```

**Manual tests:**
```bash
# Test 1: PubSub enabled on embedded IPFS
./ipfs-publisher --ipfs-mode embedded
# Expect logs: "PubSub enabled on embedded IPFS node"

# Test 2: Subscribe from external IPFS
ipfs pubsub sub mdn/collections/announce &

# Test 3: Publish test message
./ipfs-publisher --ipfs-mode embedded --test-pubsub
# Expect message received in subscriber

# Test 4: Message format validation
# Capture message and verify JSON structure
# Expect all required fields present

# Test 5: Signature verification
# Provide verification script
./verify-signature.sh <pubsub_message>
# Expect: "✓ Signature valid"

# Test 6: Peer discovery for PubSub
# Check embedded node logs for PubSub peers
# Expect: "PubSub peers on topic: X"

# Test 7: Message delivery
# Start multiple subscribers
# Publish from embedded mode
# Expect: All subscribers receive message
```

**Readiness criteria:**
- ✓ PubSub enabled on embedded IPFS node
- ✓ Messages published successfully
- ✓ Message format correct
- ✓ Signatures valid
- ✓ Peer discovery works
- ✓ No separate PubSub node created

---

### Phase 5: Directory scanning and index creation (2-3 days)
**Goal**: Scan directories and create NDJSON index

**Tasks:**
1. Implement directory scanner
2. Filter by extensions
3. Create NDJSON index
4. Upload all files to IPFS (mode-aware)
5. Upload index to IPFS
6. Progress bar for large batches

**New files:**
```
internal/
├── scanner/
│   └── scanner.go
├── index/
│   ├── manager.go
│   └── ndjson.go
```

**Manual tests:**
```bash
# Prepare test directory
mkdir -p ~/test-media
cp some-files.mp3 ~/test-media/
cp some-video.mkv ~/test-media/

# Test 1: Initial scan (dry-run) with external mode
./ipfs-publisher --dry-run --ipfs-mode external
# Expect: list of found files, no uploads

# Test 2: Initial scan (dry-run) with embedded mode
./ipfs-publisher --dry-run --ipfs-mode embedded
# Expect: same list of files

# Test 3: Upload all files (external mode)
./ipfs-publisher --ipfs-mode external
# Expect:
# - Progress bar with percent
# - Logs per file
# - Creation of ~/.ipfs_publisher/collection.ndjson

# Test 4: Upload all files (embedded mode)
./ipfs-publisher --ipfs-mode embedded
# Expect: same behavior as external mode

# Test 5: Check index contents
cat ~/.ipfs_publisher/collection.ndjson
# Expect lines like:
# {"id":1,"CID":"QmXxx...","filename":"file1.mp3","extension":"mp3"}

# Test 6: Index uploaded to IPFS
INDEX_CID=$(cat ~/.ipfs_publisher/state.json | jq -r .lastIndexCID)
ipfs cat $INDEX_CID
# Expect: NDJSON content

# Test 7: Filtering
touch ~/test-media/ignored.txt
./ipfs-publisher --dry-run
# Expect: ignored.txt not listed

# Test 8: Large batch (>100 files)
for i in {1..150}; do touch ~/test-media/file-$i.mp3; done
./ipfs-publisher
# Expect: working progress bar with ETA
```

**Readiness criteria:**
- ✓ Files from configured directories found
- ✓ Extension filtering works
- ✓ NDJSON index created correctly
- ✓ Files uploaded to IPFS with correct CIDs
- ✓ Index uploaded to IPFS
- ✓ Progress bar works
- ✓ Works in both IPFS modes

---

### Phase 6: IPNS and key management (2 days)
**Goal**: Generate keys and publish index through IPNS

**Tasks:**
1. Ed25519 key pair generation
2. Save keys to disk with correct permissions
3. Create IPNS record (mode-aware)
4. Update IPNS on changes
5. Integration with state management

**New files:**
```
internal/
├── keys/
│   └── manager.go
├── ipns/
│   └── manager.go
```

**Manual tests:**
```bash
# Test 1: Key generation on first run
rm -rf ~/.ipfs_publisher/keys
./ipfs-publisher
# Expect: creation of private.key and public.key

# Test 2: Key permissions
ls -la ~/.ipfs_publisher/keys/
# Expect: private.key with 0600, directory with 0700

# Test 3: IPNS publish (external mode)
./ipfs-publisher --ipfs-mode external
IPNS_HASH=$(cat ~/.ipfs_publisher/state.json | jq -r .ipns)
ipfs name resolve $IPNS_HASH
# Expect: index CID

# Test 4: IPNS publish (embedded mode)
./ipfs-publisher --ipfs-mode embedded
IPNS_HASH=$(cat ~/.ipfs_publisher/state.json | jq -r .ipns)
# Resolve from external node
ipfs name resolve $IPNS_HASH
# Expect: index CID (may take time to propagate via DHT)

# Test 5: Fetch index via IPNS
ipfs cat $IPNS_HASH
# Expect: NDJSON content

# Test 6: Update collection and IPNS
cp new-file.mp3 ~/test-media/
./ipfs-publisher
ipfs name resolve $IPNS_HASH
# Expect: new CID

# Test 7: Use existing keys
./ipfs-publisher
# Expect: "Loaded existing IPNS keypair"

# Test 8: IPNS in state file
cat ~/.ipfs_publisher/state.json | jq .
# Expect: ipns field with k51... hash
```

**Readiness criteria:**
- ✓ Keys generated on first run
- ✓ Keys loaded on subsequent runs
- ✓ Correct file permissions
- ✓ Index uploaded to IPFS
- ✓ IPNS record created
- ✓ IPNS points to index
- ✓ IPNS updates on changes
- ✓ Works in both IPFS modes

---

### Phase 7: Complete PubSub integration for embedded mode (1-2 days)
**Goal**: Full PubSub announcement flow using embedded IPFS node

**Tasks:**
1. Integrate PubSub with index updates
2. Implement periodic announcements
3. Version counter management
4. Message signing with IPNS keys
5. Error handling and retries

**Manual tests:**
```bash
# Test 1: Subscribe and monitor
ipfs pubsub sub mdn/collections/announce &

# Test 2: First publish after initial scan
./ipfs-publisher --ipfs-mode embedded
# Expect: PubSub message with version=1

# Test 3: Add file triggers publish
cp new-song.mp3 ~/test-media/
# Expect: PubSub message with version=2

# Test 4: Message content validation
# Capture last message
# Verify: version, ipns, publicKey, collectionSize, timestamp, signature

# Test 5: Periodic announcements
# Wait 60+ minutes or reduce interval in config
# Expect: Repeated messages with same version if no changes

# Test 6: Timestamp behavior
# Note timestamp from first message
# Wait for periodic announcement
# Expect: timestamp unchanged if no collection changes

# Test 7: Signature verification
./verify-signature.sh <captured_message>
# Expect: signature valid with publicKey from message

# Test 8: PubSub uses embedded IPFS
# Verify in logs that no separate PubSub node is created
# Expect: "Using embedded IPFS node for PubSub"
```

**Readiness criteria:**
- ✓ Messages published on collection changes
- ✓ Periodic announcements work
- ✓ Message format matches spec
- ✓ Signatures verifiable
- ✓ Version increments correctly
- ✓ Timestamp behavior correct
- ✓ Uses embedded IPFS libp2p instance
- ✓ No separate PubSub node created

---

### Phase 7.1: Standalone PubSub for External IPFS Mode (2-3 days)
**Goal**: Implement standalone libp2p PubSub node for external IPFS mode

**Context**: Phase 7 implemented PubSub for embedded IPFS mode (using the same libp2p instance). Now we need to add support for external IPFS mode where a separate lightweight PubSub node is required.

**Tasks:**
1. Create standalone libp2p node for PubSub only
2. Implement GossipSub protocol on standalone node
3. Configure separate swarm port for PubSub node
4. Use same bootstrap peers as IPFS for network connectivity
5. Enable DHT for peer discovery and topic announcement
6. Integrate standalone PubSub with PubsubPublisher (mode detection)
7. Lifecycle management (start/stop with application)
8. Monitor peer connections and DHT status

**New files:**
```
internal/
├── pubsub/
│   ├── standalone.go    # Standalone libp2p PubSub node (external mode)
│   ├── embedded.go      # Wrapper for embedded IPFS PubSub (embedded mode)
│   └── publisher.go     # Mode-aware publisher (updated)
```

**Manual tests:**
```bash
# Prepare: start external IPFS daemon
ipfs daemon &

# Test 1: Standalone PubSub node starts in external mode
./ipfs-publisher --ipfs-mode external
# Expect logs: 
# - "Starting standalone PubSub node on port 4003"
# - "PubSub node peer ID: 12D3Koo..."
# - "Connected to X bootstrap peers"

# Test 2: No standalone node in embedded mode
./ipfs-publisher --ipfs-mode embedded
# Expect logs:
# - "Using embedded IPFS node for PubSub"
# - NO logs about standalone PubSub node

# Test 3: Subscribe from external IPFS
ipfs pubsub sub mdn/collections/announce &

# Test 4: Publish from external mode
./ipfs-publisher --ipfs-mode external --test-pubsub
# Expect: message received in ipfs subscriber

# Test 5: Cross-mode communication
# Terminal 1: embedded mode
./ipfs-publisher --ipfs-mode embedded &
# Terminal 2: external mode  
./ipfs-publisher --ipfs-mode external &
# Both should see each other's announcements in logs

# Test 6: DHT peer discovery
./ipfs-publisher --ipfs-mode external
# Wait 2-3 minutes
# Check logs for DHT bootstrap and peer discovery
# Expect: "DHT routing table: X peers"

# Test 7: Standalone node resource usage
./ipfs-publisher --ipfs-mode external
ps aux | grep ipfs-publisher
# Monitor memory/CPU
# Expect: minimal overhead from standalone PubSub (~20-50MB)

# Test 8: Port configuration
# Change pubsub.standalone.swarm_port in config to 14003
./ipfs-publisher --ipfs-mode external
netstat -tuln | grep 14003
# Expect: port 14003 listening

# Test 9: Bootstrap peer connectivity
./ipfs-publisher --ipfs-mode external --config custom-bootstrap.yaml
# Custom config with specific bootstrap peers
# Expect: connects to specified peers

# Test 10: Graceful shutdown
./ipfs-publisher --ipfs-mode external
# Send SIGTERM or Ctrl+C
# Expect: 
# - "Shutting down standalone PubSub node..."
# - Clean DHT provider cleanup
# - No hanging goroutines

# Test 11: Standalone node restart on failure
# In external mode, manually kill standalone PubSub process (if detectable)
# Expect: automatic restart attempt with error logs

# Test 12: Topic subscription verification
./ipfs-publisher --ipfs-mode external
# From another terminal:
ipfs pubsub peers mdn/collections/announce
# Expect: standalone PubSub node's peer ID in list

# Test 13: Message delivery rate
# Run both modes, trigger frequent updates
# Verify both embedded and external modes receive all messages
# Expect: no message loss, consistent delivery

# Test 14: Bootstrap failure handling
# Configure invalid bootstrap peers
./ipfs-publisher --ipfs-mode external
# Expect: 
# - Error logs about bootstrap failure
# - Retry attempts
# - Application continues (degraded mode)

# Test 15: Same network verification
# External IPFS node peer ID: ipfs id
# Standalone PubSub peer ID: from logs
# Both should discover same DHT peers
# Expect: overlap in peer lists (use ipfs dht findpeer)
```

**Readiness criteria:**
- ✓ Standalone PubSub node starts in external mode only
- ✓ No standalone node created in embedded mode
- ✓ Separate peer identity for standalone node
- ✓ Custom swarm port configurable
- ✓ Uses same bootstrap peers as IPFS
- ✓ DHT enabled and functional
- ✓ Messages published successfully from both modes
- ✓ Cross-mode message delivery works
- ✓ Graceful shutdown and cleanup
- ✓ Minimal resource overhead
- ✓ Peer discovery functional
- ✓ Mode switching works correctly

---

### Phase 8: File watcher and state management (2-3 days)
**Goal**: Real-time file change detection and state persistence

**Tasks:**
1. Integrate `fsnotify`
2. Handle create/modify/delete/rename events
3. Incremental index updates
4. Debouncing for frequent changes
5. State save and restore
6. Recovery after crashes

**New files:**
```
internal/
├── watcher/
│   └── watcher.go
├── state/
│   └── manager.go
```

**Manual tests:**
```bash
# Test 1: Run in watch mode
./ipfs-publisher

# Test 2: Add new file
cp new-song.mp3 ~/test-media/
# Expect: automatic processing and PubSub update

# Test 3: Modify file
echo "updated" >> ~/test-media/existing.mp3
# Expect: reupload, index update, version increment

# Test 4: Delete file
rm ~/test-media/old-file.mp3
# Expect: removal from index, version increment

# Test 5: Rename file
mv ~/test-media/song.mp3 ~/test-media/renamed.mp3
# Expect: filename updated, id and CID preserved

# Test 6: Debounce rapid changes
for i in 1 2 3 4 5; do
  echo "change $i" >> ~/test-media/test.mp3
  sleep 0.1
done
# Expect: only one processing after 300ms debounce

# Test 7: State persistence
cat ~/.ipfs_publisher/state.json | jq .
# Expect: version, ipns, lastIndexCID, files with metadata

# Test 8: Recovery after crash
./ipfs-publisher
# Add files, kill -9 process
./ipfs-publisher
# Expect: state loaded, missing files reprocessed

# Test 9: Ignore patterns
echo "test" > ~/test-media/.hidden
echo "test" > ~/test-media/file~
# Expect: ignored in logs

# Test 10: State in both IPFS modes
# Test state persistence with external mode
# Test state persistence with embedded mode
# Expect: state format identical
```

**Readiness criteria:**
- ✓ New files detected automatically
- ✓ Changes processed correctly
- ✓ Deletions handled
- ✓ Renames preserve id/CID
- ✓ Debounce works
- ✓ State persists and loads
- ✓ Recovery after crashes works
- ✓ Ignore patterns work
- ✓ Works in both IPFS modes

---

### Phase 9: Final polish and edge cases (2-3 days)
**Goal**: Handle edge cases, improve UX, optimize performance

**Tasks:**
1. Improve error messages and logging
2. Add comprehensive help and documentation
3. Handle all edge cases from PRD
4. Performance tuning for large collections
5. Memory leak checks
6. Resource cleanup
7. Mode-specific optimizations

**Manual tests:**
```bash
# Test 1: Help and documentation
./ipfs-publisher --help
./ipfs-publisher --version

# Test 2: Init command
./ipfs-publisher --init
# Expect: config file created, keys generated

# Test 3: Special characters in filenames
touch ~/test-media/"file with spaces.mp3"
touch ~/test-media/"файл-кириллица.mp3"
touch ~/test-media/"file'with\"quotes.mp3"
./ipfs-publisher

# Test 4: Very long filenames
touch ~/test-media/"$(printf 'a%.0s' {1..300}).mp3"
./ipfs-publisher
# Expect: handled gracefully (truncated or error logged)

# Test 5: Symlinks
ln -s ~/other-dir ~/test-media/symlink
./ipfs-publisher --dry-run
# Expect: ignored or followed (document behavior)

# Test 6: Large collection stress test
for i in {1..1000}; do
  touch ~/test-media/file-$i.mp3
done
./ipfs-publisher
# Expect: completes successfully, reasonable memory usage

# Test 7: Resource monitoring during long run
./ipfs-publisher &
# Monitor with: watch -n 5 'ps aux | grep ipfs-publisher'
# Add/remove files periodically for 1 hour
# Expect: stable memory, no leaks

# Test 8: Graceful shutdown during operations
./ipfs-publisher
# While uploading files, press Ctrl+C
# Expect: "Shutting down gracefully...", state saved, lock removed

# Test 9: Mode switching
# Run with external mode
./ipfs-publisher --ipfs-mode external
# Stop, switch config to embedded
./ipfs-publisher --ipfs-mode embedded
# Expect: state preserved, continues from where it left off

# Test 10: Embedded node resource usage
# Monitor embedded node:
./ipfs-publisher --ipfs-mode embedded
# Check: memory, CPU, disk I/O, peer connections
# Expect: reasonable resource usage

# Test 11: Configuration validation
# Invalid IPFS mode
# Invalid ports (negative, out of range)
# Expect: clear error messages

# Test 12: Debug logs
./ipfs-publisher --config config.yaml
# Set logging.level: debug in config
# Expect: detailed logs for troubleshooting
```

**Readiness criteria:**
- ✓ All edge cases covered
- ✓ Clear and helpful error messages
- ✓ Comprehensive documentation
- ✓ Performance acceptable (per acceptance criteria)
- ✓ No memory leaks
- ✓ Graceful shutdown works
- ✓ Both IPFS modes thoroughly tested
- ✓ Resource usage within bounds

---

### Phase 10: Integration testing and production readiness (1-2 days)
**Goal**: End-to-end testing and final validation

**Tasks:**
1. Complete integration test suite
2. Test all mode combinations
3. Long-running stability tests
4. Document deployment procedures
5. Create troubleshooting guide
6. Prepare release artifacts

**Manual tests:**
```bash
# Test 1: Complete workflow (external mode)
./ipfs-publisher --ipfs-mode external --init
# Add test directory
# Wait for full cycle: scan -> upload -> IPNS -> PubSub
# Verify all components work

# Test 2: Complete workflow (embedded mode)
./ipfs-publisher --ipfs-mode embedded --init
# Same as Test 1
# Verify embedded node operates correctly

# Test 3: 24-hour stability test
./ipfs-publisher &
# Run for 24+ hours with periodic file changes
# Monitor logs, memory, CPU
# Expect: stable operation, no crashes

# Test 4: Network interruption handling
./ipfs-publisher --ipfs-mode external
# Disconnect network during operation
# Expect: errors logged, retries, recovery on reconnect

# Test 5: Embedded node crash recovery
./ipfs-publisher --ipfs-mode embedded
# Kill embedded IPFS process manually
# Expect: detected, logged, node restarted

# Test 6: Large file handling
cp large-file-2GB.mkv ~/test-media/
./ipfs-publisher
# Expect: uploads successfully without OOM

# Test 7: Concurrent changes
# Script to continuously modify files
while true; do
  echo "update" >> ~/test-media/test-$RANDOM.mp3
  sleep 1
done
./ipfs-publisher
# Expect: handles continuous changes

# Test 8: Migration between modes
# Start with external mode, populate collection
./ipfs-publisher --ipfs-mode external
# Stop, switch to embedded
./ipfs-publisher --ipfs-mode embedded
# Expect: collection state preserved, continues operation

# Test 9: Clean installation
# Fresh system, no prior config
./ipfs-publisher --init
./ipfs-publisher
# Expect: all setup automatic, works out of box

# Test 10: Production deployment simulation
# Deploy as systemd service
sudo systemctl start ipfs-publisher
sudo systemctl status ipfs-publisher
# Expect: runs as service, logs to journal
```

**Readiness criteria:**
- ✓ All integration tests pass
- ✓ 24-hour stability test successful
- ✓ Both IPFS modes production-ready
- ✓ Documentation complete
- ✓ Troubleshooting guide created
- ✓ Deployment procedures documented
- ✓ Release artifacts prepared

---

## 14. Final Production Readiness Checklist

### Functionality
- [ ] Scan multiple directories
- [ ] Filter by extensions
- [ ] Upload files to IPFS (external mode)
- [ ] Upload files to IPFS (embedded mode)
- [ ] Create NDJSON index
- [ ] IPNS publish (external mode)
- [ ] IPNS publish (embedded mode)
- [ ] PubSub announcements (always embedded)
- [ ] Real-time change monitoring
- [ ] Incremental updates
- [ ] State save and restore
- [ ] Mode switching support

### IPFS Integration
- [ ] External IPFS connection works
- [ ] Embedded IPFS node starts successfully
- [ ] Port conflict detection works
- [ ] Embedded node repo persistence
- [ ] PubSub works on embedded IPFS node (embedded mode)
- [ ] Standalone PubSub node works (external mode)
- [ ] DHT integration (both modes)
- [ ] Garbage collection (embedded mode)
- [ ] Bootstrap peer connectivity (both IPFS and PubSub)

### Reliability
- [ ] Lock file prevents multiple runs
- [ ] Graceful shutdown (both modes)
- [ ] Handle IPFS unavailability (external)
- [ ] Handle embedded node failures
- [ ] Retry logic for operations
- [ ] Handle files deleted during processing
- [ ] Correct recovery after crash
- [ ] State integrity maintained

### UX
- [ ] Progress bar for large collections
- [ ] Clear logs with mode indicators
- [ ] `--help` documentation
- [ ] `--dry-run` for testing
- [ ] `--init` to create config
- [ ] `--ipfs-mode` override flag
- [ ] YAML configuration
- [ ] Port conflict error messages
- [ ] Mode selection guidance

### Security
- [ ] Correct permissions for private keys (0600)
- [ ] Keys directory permissions (0700)
- [ ] Signed PubSub messages
- [ ] Path validation
- [ ] Filename sanitization
- [ ] Embedded node security considerations

### Performance
- [ ] < 500MB memory for 10k files (external)
- [ ] < 1GB memory for 10k files (embedded)
- [ ] Debouncing for frequent changes
- [ ] Streaming index processing
- [ ] No memory leaks (24h+ test)
- [ ] Embedded node resource usage acceptable

### Documentation
- [ ] README with mode selection guide
- [ ] Config format documentation
- [ ] IPFS mode comparison table
- [ ] Port configuration guide
- [ ] Troubleshooting guide (mode-specific)
- [ ] Usage examples (both modes)
- [ ] Deployment guide
- [ ] Migration guide between modes

---

## 15. IPFS Mode Comparison Table

| Feature | Embedded Mode | External Mode |
|---------|--------------|---------------|
| **IPFS Node** | Built-in, full node | External daemon required |
| **Dependencies** | None | IPFS daemon must be running |
| **Setup Complexity** | Simple (automatic) | Medium (manual daemon setup) |
| **Resource Usage** | Higher (~500MB-1GB) | Lower (~200-300MB) |
| **PubSub Node** | Same as IPFS | Separate standalone node |
| **Port Requirements** | 3 ports (swarm, API, gateway) + PubSub uses same swarm | External IPFS ports + 1 PubSub port |
| **DHT Participation** | Full DHT node | External IPFS + standalone PubSub DHT |
| **Content Availability** | High (own node) | Depends on external node |
| **Bootstrap** | Automatic | Inherits from external |
| **Pinning Control** | Full control | Via external node |
| **Best For** | Production, standalone | Development, testing |
| **Startup Time** | ~10-30 seconds | Instant (if daemon running) |
| **Network Identity** | Own peer ID | Uses external + standalone PubSub |
| **Garbage Collection** | Configurable | External node controls |
| **failover** | Self-contained | Depends on external daemon |

---

## 16. Configuration Examples

### Example 1: Embedded Mode (Default)
```yaml
ipfs:
  mode: "embedded"
  
  embedded:
    repo_path: "~/.ipfs_publisher/ipfs-repo"
    swarm_port: 4002
    api_port: 5002
    gateway_port: 8081
    
    add_options:
      pin: true
      chunker: "size-262144"
    
    gc:
      enabled: true
      interval: 86400
      
pubsub:
  topic: "mdn/collections/announce"
  announce_interval: 3600

directories:
  - "/home/user/media"

extensions:
  - "mp3"
  - "mkv"
```

### Example 2: External Mode (Development)
```yaml
ipfs:
  mode: "external"
  
  external:
    api_url: "http://localhost:5001"
    timeout: 300
    add_options:
      nocopy: true  # Uses filestore
      pin: true
      chunker: "size-262144"
      
pubsub:
  topic: "mdn/collections/announce"
  announce_interval: 3600
  
  # Standalone PubSub node (used in external mode)
  standalone:
    bootstrap_peers: []  # Uses standard IPFS bootstrap
    swarm_port: 4003
    enable_dht: true

directories:
  - "/data/media"

extensions:
  - "mp3"
  - "mkv"
  - "mp4"
```

### Example 3: Production Embedded Mode
```yaml
ipfs:
  mode: "embedded"
  
  embedded:
    repo_path: "/var/lib/ipfs-publisher/repo"
    swarm_port: 14001
    api_port: 15001
    gateway_port: 18080
    
    add_options:
      pin: true
      chunker: "size-1048576"  # 1MB chunks for large files
      raw_leaves: true
    
    gc:
      enabled: true
      interval: 43200  # 12 hours
      min_free_space: 5368709120  # 5GB

pubsub:
  topic: "production/media/announce"
  announce_interval: 1800  # 30 minutes
  # No standalone section needed - embedded IPFS handles PubSub

directories:
  - "/mnt/storage/media"

extensions:
  - "mp3"
  - "flac"
  - "mkv"
  - "mp4"

logging:
  level: "info"
  file: "/var/log/ipfs-publisher/app.log"
  max_size: 500
  max_backups: 10

behavior:
  scan_interval: 5
  batch_size: 20
  state_save_interval: 30
```
