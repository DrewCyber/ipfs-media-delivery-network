# IPFS Content Indexer

An automated service for indexing IPFS content announced through PubSub. This indexer subscribes to collection announcements, downloads content via IPNS, and maintains a searchable SQLite database.

## Features

- **Embedded IPFS Node**: Runs Kubo node internally with customizable ports
- **PubSub Subscription**: Listens to collection announcements on configurable topics
- **Automatic Fetching**: Downloads collections via IPNS with retry mechanism (10 attempts, 1-minute intervals)
- **SQLite Database**: Stores hosts, publishers, collections, and content index
- **Database Migrations**: Automatic schema management using goose
- **Concurrent Downloads**: Configurable parallel collection fetching (default: 5)
- **Graceful Shutdown**: Handles SIGTERM/SIGINT with proper cleanup

## Architecture

```
┌─────────────────┐
│   IPFS PubSub   │
│     Topic       │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────┐
│   Indexer Service           │
│  ┌─────────────────────┐    │
│  │ Embedded IPFS Node  │    │
│  └─────────────────────┘    │
│  ┌─────────────────────┐    │
│  │ PubSub Listener     │    │
│  └─────────────────────┘    │
│  ┌─────────────────────┐    │
│  │ Collection Fetcher  │    │
│  └─────────────────────┘    │
│  ┌─────────────────────┐    │
│  │ Parser & Validator  │    │
│  └─────────────────────┘    │
└──────────┬──────────────────┘
           │
           ▼
    ┌──────────────┐
    │  SQLite DB   │
    └──────────────┘
```

## Installation

### Prerequisites

- Go 1.25 or higher
- Ports available: 4003 (swarm), 5003 (API), 8082 (gateway)

### Build

```bash
cd apps/indexer
GOWORK=off go build -o ipfs-indexer ./cmd/ipfs-indexer
```

## Configuration

Edit `config.yaml` to customize settings:

```yaml
database:
  type: "sqlite"
  path: "./data/indexer.db"

ipfs:
  mode: "embedded"
  embedded:
    repo_path: "./ipfs_indexer_repo"
    swarm_port: 4003
    api_port: 5003
    gateway_port: 8082
    bootstrap_peers: []
    gc:
      enabled: true
      interval: 86400
      min_free_space: 1073741824

pubsub:
  topic: "ipfs-collections-index"

fetcher:
  retry_attempts: 10
  retry_interval_seconds: 60
  concurrent_downloads: 5

logging:
  level: "info"
  format: "text"
  output: "stdout"
  file_path: "./logs/indexer.log"
```

## Usage

### Start the Indexer

```bash
./ipfs-indexer
```

Or with custom config:

```bash
./ipfs-indexer -config /path/to/config.yaml
```

### Database Schema

The indexer maintains the following tables:

- **hosts**: IPFS nodes that sent PubSub messages
- **publishers**: Owners of IPNS keys
- **collections**: Collection announcements with status tracking
- **index_items**: Individual content items (CID, filename, extension)

### PubSub Message Format

The indexer expects messages in this format:

```json
{
  "version": 1,
  "ipns": "k2k4r8ltgwjllr3n1on4rwis0kc853wzdcyjgt5xk2lcui5xn95c5vl2",
  "publicKey": "E8WtP2ctD8iOoZ1s95xrU55a4iYaCdlUD+auyMZfPLM=",
  "collectionSize": 4,
  "timestamp": 1764260509,
  "signature": "XoFDGnjThpqJnmh0/c8nERCOxNjly20007VqZAqpaUnZ5m5VGsIUjIBFYu/W62c5IQ4qDaM5ysHQJVK7jkAyAg=="
}
```

### Collection File Format (JSONL)

Collections should be in JSON Lines format:

```
{"id":2,"CID":"QmaYsXFBVpMMk74Ed78342XSH26wQZs9Y8PyAUWNCxzyZp","filename":"test-15mb.mp3","extension":"mp3"}
{"id":7,"CID":"QmepHP9vMsBZB7w15yEqnUzTupNoQqnG9Lj3VhBQAvxg6B","filename":"song.mp3","extension":"mp3"}
```

## Status Tracking

Collections go through the following states:

- **pending**: Waiting to be fetched
- **downloaded**: Successfully fetched and indexed
- **failed**: Failed after maximum retry attempts (10)

## Retry Mechanism

- Failed downloads are retried up to 10 times
- 60-second interval between retries
- After 10 failed attempts, collection is marked as "failed"

## Logging

Log levels: `debug`, `info`, `warn`, `error`

Logs include:
- PubSub message receipts
- Collection fetch attempts
- Parsing results
- Database operations
- Error details

## Future Enhancements (Not in Phase 1)

- Quickwit integration for full-text search
- IPNS signature validation
- Content validation (CID availability checks)
- Publisher reputation system
- Rate limiting
- Content deduplication
- REST API

## License

See root LICENSE file.
