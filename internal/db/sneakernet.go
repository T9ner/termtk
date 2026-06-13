package db

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SyncFile represents the file structure exported for offline message transfer.
type SyncFile struct {
	SenderUUID     string    `json:"sender_uuid"`
	SenderUsername string    `json:"sender_username"`
	ExportedAt     time.Time `json:"exported_at"`
	Messages       []Message `json:"messages"`
}

// ExportSyncFile exports all unsynced messages between the local user and a contact to a JSON file.
func (d *Database) ExportSyncFile(contactUUID string, localProfile *Profile, exportPath string) error {
	// 1. Get messages that are not synced yet
	unsynced, err := d.GetUnsyncedMessages(contactUUID)
	if err != nil {
		return fmt.Errorf("failed to retrieve unsynced messages: %w", err)
	}

	if len(unsynced) == 0 {
		return fmt.Errorf("no unsynced messages to export")
	}

	// 2. Prepare file payload
	payload := SyncFile{
		SenderUUID:     localProfile.UUID,
		SenderUsername: localProfile.Username,
		ExportedAt:     time.Now(),
		Messages:       unsynced,
	}

	// 3. Serialize and write
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode sync payload: %w", err)
	}

	if err := os.WriteFile(exportPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sync file: %w", err)
	}

	// 4. Mark exported messages as synced
	for _, m := range unsynced {
		if err := d.UpdateMessageStatus(m.ID, "synced"); err != nil {
			return fmt.Errorf("failed to update message status after export: %w", err)
		}
	}

	return nil
}

// ImportSyncFile imports and merges messages from a sync file into the local database.
func (d *Database) ImportSyncFile(importPath string) (*SyncFile, error) {
	// 1. Read file
	data, err := os.ReadFile(importPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sync file: %w", err)
	}

	// 2. Parse payload
	var payload SyncFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode sync file JSON: %w", err)
	}

	// 3. Register the sender as a contact if they don't exist
	senderContact := &Contact{
		UUID:     payload.SenderUUID,
		Username: payload.SenderUsername,
		IP:       "offline",
		Port:     0,
		LastSeen: time.Now(),
	}
	if err := d.UpsertContact(senderContact); err != nil {
		return nil, fmt.Errorf("failed to register contact during import: %w", err)
	}

	// 4. Import messages
	for i := range payload.Messages {
		msg := &payload.Messages[i]

		// Integrity validation: verify the hash matches content
		if msg.GenerateID() != msg.ID {
			return nil, fmt.Errorf("message integrity violation: invalid hash for message ID %s", msg.ID)
		}

		// Save/merge into the local database (upsert checks for duplicates)
		msg.Status = "synced"
		if err := d.SaveMessage(msg); err != nil {
			return nil, fmt.Errorf("failed to save message %s during import: %w", msg.ID, err)
		}
	}

	return &payload, nil
}
