package database

import (
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// DB wraps the database connection
type DB struct {
	conn *sql.DB
	log  *logrus.Logger
}

// New creates a new database connection and runs migrations
func New(dbPath string, log *logrus.Logger) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		conn: conn,
		log:  log,
	}

	// Run migrations
	if err := db.runMigrations(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Info("Database initialized successfully")
	return db, nil
}

// runMigrations runs all pending migrations
func (db *DB) runMigrations() error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}

	if err := goose.Up(db.conn, "migrations"); err != nil {
		return err
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// GetConn returns the underlying database connection
func (db *DB) GetConn() *sql.DB {
	return db.conn
}

// Host represents a host (IPFS node that sent PubSub message)
type Host struct {
	ID        int64
	PublicKey string
	CreatedAt string
}

// Publisher represents a publisher (owner of IPNS key)
type Publisher struct {
	ID        int64
	PublicKey string
	CreatedAt string
}

// Collection represents a collection announcement
type Collection struct {
	ID          int64
	HostID      int64
	PublisherID int64
	Version     int
	IPNS        string
	Size        *int
	Timestamp   int64
	Status      string
	RetryCount  int
	LastRetryAt *string
	CreatedAt   string
	UpdatedAt   string
}

// IndexItem represents a content item in the index
type IndexItem struct {
	ID           int64
	CID          string
	Filename     string
	Extension    string
	HostID       int64
	PublisherID  int64
	CollectionID int64
	CreatedAt    string
	UpdatedAt    string
}

// CreateOrGetHost creates a new host or returns existing one
func (db *DB) CreateOrGetHost(publicKey string) (*Host, error) {
	var host Host

	// Try to get existing host
	err := db.conn.QueryRow(`
		SELECT id, public_key, created_at 
		FROM hosts 
		WHERE public_key = ?
	`, publicKey).Scan(&host.ID, &host.PublicKey, &host.CreatedAt)

	if err == nil {
		return &host, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query host: %w", err)
	}

	// Create new host
	result, err := db.conn.Exec(`
		INSERT INTO hosts (public_key) VALUES (?)
	`, publicKey)

	if err != nil {
		return nil, fmt.Errorf("failed to insert host: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	host.ID = id
	host.PublicKey = publicKey

	return &host, nil
}

// CreateOrGetPublisher creates a new publisher or returns existing one
func (db *DB) CreateOrGetPublisher(publicKey string) (*Publisher, error) {
	var publisher Publisher

	// Try to get existing publisher
	err := db.conn.QueryRow(`
		SELECT id, public_key, created_at 
		FROM publishers 
		WHERE public_key = ?
	`, publicKey).Scan(&publisher.ID, &publisher.PublicKey, &publisher.CreatedAt)

	if err == nil {
		return &publisher, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query publisher: %w", err)
	}

	// Create new publisher
	result, err := db.conn.Exec(`
		INSERT INTO publishers (public_key) VALUES (?)
	`, publicKey)

	if err != nil {
		return nil, fmt.Errorf("failed to insert publisher: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	publisher.ID = id
	publisher.PublicKey = publicKey

	return &publisher, nil
}

// CreateCollection creates a new collection
func (db *DB) CreateCollection(hostID, publisherID int64, version int, ipns string, size *int, timestamp int64) (*Collection, error) {
	result, err := db.conn.Exec(`
		INSERT INTO collections (host_id, publisher_id, version, ipns, size, timestamp, status)
		VALUES (?, ?, ?, ?, ?, ?, 'pending')
	`, hostID, publisherID, version, ipns, size, timestamp)

	if err != nil {
		return nil, fmt.Errorf("failed to insert collection: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &Collection{
		ID:          id,
		HostID:      hostID,
		PublisherID: publisherID,
		Version:     version,
		IPNS:        ipns,
		Size:        size,
		Timestamp:   timestamp,
		Status:      "pending",
	}, nil
}

// GetPendingCollections returns all collections with pending status and retry count < max
func (db *DB) GetPendingCollections(maxRetries int) ([]*Collection, error) {
	rows, err := db.conn.Query(`
		SELECT id, host_id, publisher_id, version, ipns, size, timestamp, status, retry_count, last_retry_at, created_at, updated_at
		FROM collections
		WHERE status = 'pending' AND retry_count < ?
		ORDER BY created_at ASC
	`, maxRetries)

	if err != nil {
		return nil, fmt.Errorf("failed to query pending collections: %w", err)
	}
	defer rows.Close()

	var collections []*Collection
	for rows.Next() {
		var c Collection
		err := rows.Scan(&c.ID, &c.HostID, &c.PublisherID, &c.Version, &c.IPNS, &c.Size, &c.Timestamp, &c.Status, &c.RetryCount, &c.LastRetryAt, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan collection: %w", err)
		}
		collections = append(collections, &c)
	}

	return collections, nil
}

// UpdateCollectionStatus updates the status of a collection
func (db *DB) UpdateCollectionStatus(id int64, status string, size *int) error {
	_, err := db.conn.Exec(`
		UPDATE collections 
		SET status = ?, size = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, size, id)

	if err != nil {
		return fmt.Errorf("failed to update collection status: %w", err)
	}

	return nil
}

// IncrementRetryCount increments the retry count for a collection
func (db *DB) IncrementRetryCount(id int64) error {
	_, err := db.conn.Exec(`
		UPDATE collections 
		SET retry_count = retry_count + 1, last_retry_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)

	if err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	return nil
}

// CreateOrUpdateIndexItem creates or updates an index item
func (db *DB) CreateOrUpdateIndexItem(cid, filename, extension string, hostID, publisherID, collectionID int64) error {
	// Check if item exists
	var existingID int64
	err := db.conn.QueryRow(`
		SELECT id FROM index_items 
		WHERE cid = ? AND collection_id = ?
	`, cid, collectionID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Create new item
		_, err := db.conn.Exec(`
			INSERT INTO index_items (cid, filename, extension, host_id, publisher_id, collection_id)
			VALUES (?, ?, ?, ?, ?, ?)
		`, cid, filename, extension, hostID, publisherID, collectionID)

		if err != nil {
			return fmt.Errorf("failed to insert index item: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query index item: %w", err)
	} else {
		// Update existing item
		_, err := db.conn.Exec(`
			UPDATE index_items 
			SET filename = ?, extension = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, filename, extension, existingID)

		if err != nil {
			return fmt.Errorf("failed to update index item: %w", err)
		}
	}

	return nil
}
