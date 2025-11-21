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

#### 2.2.1 Connecting to IPFS
- Connect to an IPFS node through the HTTP API
- Connection parameters come from config (URL, port)
- If the node is unavailable:
  - Log an ERROR
  - Retry connection every 30 seconds
  - Application does not exit, waits for node availability

#### 2.2.2 Uploading files to IPFS
- Upload files sequentially (one at a time)
- Support IPFS add options:
  - `--nocopy` (optional, from config)
  - `--pin` (optional, from config)
  - Other options via config
- Obtain CID for each uploaded file
- Log the process:
  - INFO: start uploading file
  - INFO: successful upload with CID
  - ERROR: upload error with details

#### 2.2.3 Progress Tracking
- Show a progress bar when processing a large number of files (>10)
- Progress bar info:
  - Current file being processed
  - Processed/total count
  - Percent complete
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
- On index updates:
  - Update the IPNS record to the new CID
  - The IPNS hash remains the same
- IPNS record TTL: 24 hours
- IPNS record is signed with the private key

### 2.5 Pubsub Announcement

#### 2.5.1 Message format
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

#### 2.5.2 Signing messages
- The signature is created over the JSON object without the `signature` field
- Algorithm: Ed25519
- Signature is Base64 encoded
- Receivers can verify the signature using `publicKey`

#### 2.5.3 Publishing to Pubsub
- Topic: configurable in config (default `mdn/collections/announce`)
- Publish occurs:
  - On the first upload of all files
  - After each collection update
  - Every hour (regardless of changes)
- On publish error:
  - Log an ERROR
  - Wait for the next attempt (after an hour or on next change)
  - Application continues running

#### 2.5.4 Periodic announcements
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

#### 2.7.1 IPFS unavailable
- Wait with periodic retry attempts (30s)
- File processing queue accumulates
- Process queue after connection is restored

#### 2.7.2 File deleted during processing
- Catch "file not found" errors when reading/uploading
- Remove file from processing queue
- Remove record from NDJSON if it existed
- Update index and publish changes

#### 2.7.3 Insufficient disk space
- Check available disk space before processing large files
- If insufficient:
  - Log ERROR with warning
  - Skip the file
  - Continue with other files

#### 2.7.4 Incorrect permissions
- Catch "permission denied" errors
- Log with the problematic file
- Skip the file and continue

## 3. Configuration

### 3.1 Configuration file
Format: YAML
Default path: `./config.yaml` or `~/.ipfs_publisher/config.yaml`

```yaml
# IPFS connection settings
ipfs:
  api_url: "http://localhost:5001"
  timeout: 300  # seconds
  add_options:
    nocopy: false
    pin: true
    chunker: "size-262144"
    raw_leaves: true

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

# Pubsub settings
pubsub:
  topic: "mdn/collections/announce"
  announce_interval: 3600  # seconds (1 hour)

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
```

## 4. Technical Architecture

### 4.1 Technology Stack
- **Language**: Go 1.21+
- **IPFS Integration**: 
  - `github.com/ipfs/go-ipfs-api` (HTTP API client)
  - or `github.com/ipfs/kubo` (for optional embedded node)
- **File System Monitoring**: `github.com/fsnotify/fsnotify`
- **Logging**: `github.com/sirupsen/logrus` or `go.uber.org/zap`
- **Configuration**: `github.com/spf13/viper`
- **Progress Bar**: `github.com/schollz/progressbar`
- **Cryptography**: `crypto/ed25519` (standard library)

### 4.2 Application Components

#### 4.2.1 Main Components
1. **FileWatcher**: Filesystem monitoring
2. **IPFSClient**: Interaction with IPFS API
3. **IndexManager**: NDJSON index management
4. **IPNSManager**: Create and update IPNS records
5. **PubsubPublisher**: Publish announcements to Pubsub
6. **StateManager**: Save and restore state
7. **KeyManager**: Key generation and management

#### 4.2.2 Data Flow
```
FileWatcher → IPFSClient → IndexManager → IPNSManager → PubsubPublisher
                ↓                                            ↓
           StateManager ←─────────────────────────────────────┘
```

### 4.3 Concurrency Model
- Main goroutine for FileWatcher
- Separate goroutine for periodic Pubsub announcements
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
- Optional authentication to the IPFS API
- Verify TLS certificates when using HTTPS
- Rate limiting to protect from DoS

## 6. Monitoring and Observability

### 6.1 Logging Levels
- **DEBUG**: Detailed info about every operation
- **INFO**: Major events (file processed, index updated)
- **WARN**: Potential problems (processing slowdowns)
- **ERROR**: Errors that do not stop the application

### 6.2 Metrics (optional)
- Number of processed files
- Total size uploaded
- Time to process files
- Number of errors by type
- IPFS node availability

### 6.3 Health Checks
- Check IPFS connectivity
- Check directory availability
- Check integrity of the state file
- Optional status endpoint

## 7. Testing Strategy

### 7.1 Unit Tests
- Tests for each component in isolation
- Mock IPFS API for testing IPFSClient
- Tests for correct signing/verification

### 7.2 Integration Tests
- Tests with a real IPFS node (testnet)
- Tests for file change scenarios
- Tests for recovery after failures

### 7.3 Performance Tests
- Tests with large collections (10000+ files)
- Tests with large files (>1GB)
- Memory leak tests for long-running operation

## 8. Future Enhancements

### 8.1 Potential Features
- Support multiple IPNS keys (per-directory)
- Web UI for monitoring
- Remote directories (SFTP, S3)
- Automatic cleaning of old versions in IPFS
- File metadata (tags, descriptions)
- Playlists and albums support

### 8.2 Optimization Opportunities
- Parallel file uploads to IPFS
- Deduplication by content hash
- Compression for the index file
- Incremental IPNS updates

## 9. Edge Cases and Limitations

### 9.1 Known Limitations
- Maximum index file size: ~100MB (IPFS block size constraints)
- Remote file versioning not supported
- No automatic rotation of IPNS keys
- No built-in replication to other IPFS nodes

### 9.2 Edge Cases
- **Rapid multiple changes**: Debounce 300ms
- **Cyclic symlinks**: Ignored during scan
- **Very long filenames**: Truncate to 255 characters
- **Special characters in filenames**: URL-encode in index
- **Duplicate filenames in different directories**: Add relative path to `filename`

## 10. Acceptance Criteria

### 10.1 Functional
- ✓ Application correctly handles adding new files
- ✓ File modification updates its CID in the index
- ✓ File deletion removes it from the index
- ✓ IPNS updates correctly on changes
- ✓ Pubsub messages are published every hour
- ✓ Application recovers state after restart

### 10.2 Non-Functional
- ✓ Processing 1000 files takes < 5 minutes (with `--nocopy`)
- ✓ Memory usage < 500MB with 10000 files
- ✓ Application recovers from IPFS unavailability in < 1 minute
- ✓ 99.9% uptime during continuous 30-day operation

## 11. Risks and Weaknesses

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

### 11.4 UX Issues
1. **No progress indication on first run**
   - Problem: User cannot see progress
   - Mitigation: Detailed progress bar with ETA

2. **Unclear why a file is not processed**
   - Problem: File can be ignored for many reasons (extension, permissions)
   - Mitigation: Explicit WARNING logs for ignored files

### 11.5 Security Issues
1. **Public key in Pubsub can be spoofed**
   - Problem: Attacker can publish a different `publicKey`
   - Mitigation: Receivers should maintain a whitelist of trusted keys

2. **No spam protection in Pubsub**
   - Problem: Attacker can flood the topic
   - Mitigation: Rate limiting on receivers

### 11.6 Operational Risks
1. **No automatic cleanup of old CIDs in IPFS**
   - Problem: IPFS node may run out of space with frequent updates
   - Mitigation: Periodic garbage collection or unpinning old versions

2. **Debugging difficulty for IPFS issues**
   - Problem: IPFS API errors may be uninformative
   - Mitigation: Verbose logging of IPFS requests/responses

## 12. Remediation Recommendations

### 12.1 High Priority (MVP)
1. Add a lock file to prevent multiple runs
2. Implement integrity check (mtime after upload)
3. Use `uint64` for the version counter
4. Implement graceful shutdown with state save

### 12.2 Medium Priority (v1.1)
1. Implement streaming index processing for large collections
2. Add index size monitoring with alerts
3. Add retry logic with exponential backoff for IPFS operations
4. Implement periodic IPNS refresh (every 12 hours)

### 12.3 Low Priority (future)
1. Optional verification of uploaded files (config flag)
2. Chunked indexes for collections >100k files
3. Add metrics and monitoring
4. Implement automatic garbage collection of old CIDs
5. Add a web UI for monitoring

## 13. Implementation Plan (Phased)

### Phase 1: Basic structure and configuration (1-2 days)
**Goal**: Set up project, configuration and basic components

**Tasks:**
1. Initialize Go module and project structure
2. Implement YAML configuration loading
3. Set up logging (file + console)
4. Implement lock file mechanism
5. Basic CLI skeleton with flags

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
```

**Readiness criteria:**
- ✓ Application starts and reads config
- ✓ Logs written to file and console
- ✓ Second instance cannot start
- ✓ Graceful shutdown on Ctrl+C

---

### Phase 2: IPFS client and basic operations (2-3 days)
**Goal**: Connect to IPFS and implement file upload

**Tasks:**
1. Implement `IPFSClient` component
2. Implement connection to IPFS API
3. Implement function to add a file to IPFS
4. Error handling and retry logic
5. Support `--nocopy` and `--pin` options

**New files:**
```
internal/
├── ipfs/
│   ├── client.go
│   └── options.go
```

**Manual tests:**
```bash
# Prepare: start IPFS daemon
ipfs daemon

# Test 1: Check connection
./ipfs-publisher --check-ipfs
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
```

**Readiness criteria:**
- ✓ Successful connection to IPFS API
- ✓ Files upload and return CIDs
- ✓ `--nocopy` mode works
- ✓ Pinning works
- ✓ Application waits for IPFS when unavailable

---

### Phase 3: Directory scanning and index creation (2-3 days)
**Goal**: Scan directories and create NDJSON index

**Tasks:**
1. Implement directory scanner
2. Filter by extensions
3. Create NDJSON index
4. Upload all files to IPFS
5. Progress bar for large batches

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

# Update config.yaml to include:
# directories:
#   - "~/test-media"
# extensions:
#   - "mp3"
#   - "mkv"

# Test 1: Initial scan (dry-run)
./ipfs-publisher --dry-run
# Expect: list of found files, no uploads

# Test 2: Upload all files
./ipfs-publisher
# Expect:
# - Progress bar with percent
# - Logs per file
# - Creation of ~/.ipfs_publisher/collection.ndjson

# Test 3: Check index contents
cat ~/.ipfs_publisher/collection.ndjson
# Expect lines like:
# {"id":1,"CID":"QmXxx...","filename":"file1.mp3","extension":"mp3"}
# {"id":2,"CID":"QmYyy...","filename":"file2.mkv","extension":"mkv"}

# Test 4: Filtering
touch ~/test-media/ignored.txt
./ipfs-publisher --dry-run
# Expect: ignored.txt not listed

# Test 5: Multiple directories
# Add another directory to config and run

# Test 6: Large batch (>100 files)
./ipfs-publisher
# Expect: working progress bar with ETA
```

**Readiness criteria:**
- ✓ Files from configured directories are found
- ✓ Extension filtering works
- ✓ NDJSON index created correctly
- ✓ Files uploaded to IPFS with correct CIDs
- ✓ Progress bar works

---

### Phase 4: IPNS and key management (2 days)
**Goal**: Generate keys and publish index through IPNS

**Tasks:**
1. Ed25519 key pair generation
2. Save keys to disk
3. Upload index to IPFS
4. Create IPNS record
5. Update IPNS on changes

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
# Expect: creation of private.key and public.key and log "Generated new IPNS keypair"

# Test 2: Use existing keys
./ipfs-publisher
# Expect: "Loaded existing IPNS keypair"

# Test 3: IPNS publish
./ipfs-publisher
cat ~/.ipfs_publisher/state.json | jq .ipns
# Expect: IPNS hash like "k51qzi5uqu5d..."

# Test 4: Resolve IPNS
IPNS_HASH=$(cat ~/.ipfs_publisher/state.json | jq -r .ipns)
ipfs name resolve $IPNS_HASH
# Expect: index CID

# Test 5: Fetch index via IPNS
ipfs cat $IPNS_HASH
# Expect: NDJSON content

# Test 6: Update collection and see IPNS change
cp new-file.mp3 ~/test-media/
./ipfs-publisher
ipfs name resolve $IPNS_HASH
# Expect: new CID

# Test 7: Key permissions
ls -la ~/.ipfs_publisher/keys/
# Expect files with correct modes
```

**Readiness criteria:**
- ✓ Keys are generated on first run
- ✓ Keys loaded on subsequent runs
- ✓ Index uploaded to IPFS
- ✓ IPNS record created and points to index
- ✓ IPNS updates on collection changes
- ✓ Correct file permissions for keys

---

### Phase 5: Pubsub publishing (1-2 days)
**Goal**: Publish announcements to Pubsub

**Tasks:**
1. Create signed Pubsub message
2. Publish to topic
3. Periodic hourly publishing
4. Increment version counter

**New files:**
```
internal/
├── pubsub/
│   ├── publisher.go
│   └── message.go
```

**Manual tests:**
```bash
# Test 1: Subscribe to topic
ipfs pubsub sub mdn/collections/announce

# Test 2: First publish
./ipfs-publisher
# Expect a JSON message in subscriber terminal

# Test 3: Signature verification
# Provide a verification script
./verify-signature.sh <pubsub_message>
# Expect: "✓ Signature valid"

# Test 4: Add file and publish
cp another.mp3 ~/test-media/
# Expect pubsub message with incremented version

# Test 5: Periodic publish
# Wait or reduce interval for test

# Test 6: Timestamp behavior
# Timestamp should not change on repeated publishes without changes

# Test 7: Pubsub error handling
# Stop IPFS daemon and watch error logs
```

**Readiness criteria:**
- ✓ Messages published to the configured topic
- ✓ Message format matches spec
- ✓ Signature verifiable
- ✓ Version increments on changes
- ✓ Timestamp unchanged for repeated publishes
- ✓ Periodic publishes work

---

### Phase 6: File watcher and incremental updates (2-3 days)
**Goal**: Real-time file change detection

**Tasks:**
1. Integrate `fsnotify`
2. Handle create/modify/delete/rename events
3. Incremental index updates
4. Debouncing for frequent changes
5. Update state

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

# Test 2: Add a new file
cp new-song.mp3 ~/test-media/
# Expect logs and processing plus pubsub update

# Test 3: Modify a file
echo "updated" >> ~/test-media/existing.mp3
# Expect reupload and index update

# Test 4: Delete a file
rm ~/test-media/old-file.mp3
# Expect removal from index and version increment

# Test 5: Rename a file
mv ~/test-media/song.mp3 ~/test-media/renamed-song.mp3
# Expect filename update, id and CID preserved

# Test 6: Debounce behavior
for i in 1 2 3 4 5 6 7 8 9 10; do
  echo "change $i" >> ~/test-media/test.mp3
  sleep 0.1
done
# Expect only one processing after debounce period

# Test 7: State file check
cat ~/.ipfs_publisher/state.json | jq .

# Test 8: Recovery after restart
./ipfs-publisher
# Add a file, then kill process and restart to verify recovery

# Test 9: Ignore temp and hidden files
echo "test" > ~/test-media/.hidden
echo "test" > ~/test-media/file.tmp
# Expect these are ignored
```

**Readiness criteria:**
- ✓ New files processed automatically
- ✓ Changes detected and processed
- ✓ Deletions handled correctly
- ✓ Renames preserve id and CID
- ✓ Debounce prevents redundant work
- ✓ State persists and recovers
- ✓ Long-running stability

---

### Phase 7: Final polish and testing (1-2 days)
**Goal**: Improve UX and handle edge cases

**Tasks:**
1. Improve logging and error messages
2. Add help and documentation
3. Handle edge cases
4. Test with large collections
5. Performance tuning

**Manual tests:**
```bash
# Test 1: Help output
./ipfs-publisher --help

# Test 2: Init config
./ipfs-publisher --init

# Test 3: Special characters in filenames
touch ~/test-media/"file with spaces.mp3"
touch ~/test-media/"файл-кириллица.mp3"
touch ~/test-media/"file'with\"quotes.mp3"
./ipfs-publisher

# Test 4: Very long filenames
touch ~/test-media/"$(printf 'a%.0s' {1..300}).mp3"
./ipfs-publisher

# Test 5: Symlinks
ln -s ~/other-dir ~/test-media/symlink-dir
./ipfs-publisher --dry-run

# Test 6: Large collection stress test
for i in (seq 1 1000)
  touch ~/test-media/file-$i.mp3
end
./ipfs-publisher

# Test 7: Long run and resource monitoring

# Test 8: Graceful shutdown
# Ctrl+C during processing and verify state saved and lock removed

# Test 9: Debug logs
./ipfs-publisher --log-level debug
```

**Readiness criteria:**
- ✓ Edge cases covered
- ✓ Clear and informative logs
- ✓ Documentation up to date
- ✓ Performance acceptable for large collections
- ✓ No memory leaks
- ✓ Graceful shutdown works

## 14. Final production readiness checklist

### Functionality
- [ ] Scan multiple directories
- [ ] Filter by extensions
- [ ] Upload files to IPFS
- [ ] Create NDJSON index
- [ ] IPNS publish
- [ ] Pubsub announcements
- [ ] Real-time change monitoring
- [ ] Incremental updates
- [ ] State save and restore

### Reliability
- [ ] Lock file prevents multiple runs
- [ ] Graceful shutdown
- [ ] Handle IPFS unavailability
- [ ] Retry logic for IPFS operations
- [ ] Handle files deleted during processing
- [ ] Correct recovery after crash

### UX
- [ ] Progress bar for large collections
- [ ] Clear logs
- [ ] `--help` documentation
- [ ] `--dry-run` for testing
- [ ] `--init` to create config
- [ ] YAML configuration

### Security
- [ ] Correct permissions for private keys (0600)
- [ ] Signed pubsub messages
- [ ] Path validation
- [ ] Filename sanitization

### Performance
- [ ] < 500MB memory for 10k files
- [ ] Debouncing for frequent changes
- [ ] Streaming index processing
- [ ] No memory leaks

### Documentation
- [ ] README with examples
- [ ] Config format documentation
- [ ] Troubleshooting guide
- [ ] Usage examples
