-- +goose Up
-- +goose StatementBegin
CREATE TABLE hosts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    public_key TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE publishers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    public_key TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE collections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id INTEGER NOT NULL,
    publisher_id INTEGER NOT NULL,
    version INTEGER NOT NULL,
    ipns TEXT NOT NULL,
    size INTEGER,
    timestamp INTEGER NOT NULL,
    status TEXT DEFAULT 'pending',
    retry_count INTEGER DEFAULT 0,
    last_retry_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (host_id) REFERENCES hosts(id),
    FOREIGN KEY (publisher_id) REFERENCES publishers(id)
);

CREATE TABLE index_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cid TEXT NOT NULL,
    filename TEXT NOT NULL,
    extension TEXT NOT NULL,
    host_id INTEGER NOT NULL,
    publisher_id INTEGER NOT NULL,
    collection_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (host_id) REFERENCES hosts(id),
    FOREIGN KEY (publisher_id) REFERENCES publishers(id),
    FOREIGN KEY (collection_id) REFERENCES collections(id)
);

CREATE INDEX idx_index_items_cid ON index_items(cid);
CREATE INDEX idx_index_items_collection ON index_items(collection_id);
CREATE INDEX idx_collections_ipns_version ON collections(ipns, version);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_collections_ipns_version;
DROP INDEX IF EXISTS idx_index_items_collection;
DROP INDEX IF EXISTS idx_index_items_cid;
DROP TABLE IF EXISTS index_items;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS publishers;
DROP TABLE IF EXISTS hosts;
-- +goose StatementEnd
