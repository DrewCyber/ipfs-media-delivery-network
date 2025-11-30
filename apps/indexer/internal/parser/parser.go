package parser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/atregu/ipfs-indexer/internal/database"
	"github.com/sirupsen/logrus"
)

// ContentItem represents a single item in the collection (JSONL format)
type ContentItem struct {
	ID        int    `json:"id"`
	CID       string `json:"CID"`
	Filename  string `json:"filename"`
	Extension string `json:"extension"`
}

// Parser handles parsing collection files
type Parser struct {
	db  *database.DB
	log *logrus.Logger
}

// NewParser creates a new parser
func NewParser(db *database.DB, log *logrus.Logger) *Parser {
	return &Parser{
		db:  db,
		log: log,
	}
}

// ParseAndStore parses a JSONL collection file and stores items in the database
func (p *Parser) ParseAndStore(collection *database.Collection, content []byte) (int, error) {
	p.log.Infof("Parsing collection ID=%d...", collection.ID)

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	itemCount := 0
	errorCount := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Parse the line as JSON
		var item ContentItem
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			p.log.Warnf("Failed to parse line %d in collection ID=%d: %v", lineNum, collection.ID, err)
			errorCount++
			continue
		}

		// Validate required fields
		if item.CID == "" || item.Filename == "" || item.Extension == "" {
			p.log.Warnf("Skipping line %d in collection ID=%d: missing required fields (CID, filename, or extension)", lineNum, collection.ID)
			errorCount++
			continue
		}

		// Store or update the item in the database
		if err := p.db.CreateOrUpdateIndexItem(
			item.CID,
			item.Filename,
			item.Extension,
			collection.HostID,
			collection.PublisherID,
			collection.ID,
		); err != nil {
			p.log.Errorf("Failed to store item from line %d in collection ID=%d: %v", lineNum, collection.ID, err)
			errorCount++
			continue
		}

		itemCount++
	}

	if err := scanner.Err(); err != nil {
		return itemCount, fmt.Errorf("error reading collection content: %w", err)
	}

	p.log.Infof("Parsed collection ID=%d: %d items stored, %d errors", collection.ID, itemCount, errorCount)

	return itemCount, nil
}
