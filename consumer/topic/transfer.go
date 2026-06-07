package topic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gauas/upload-service/consumer/service"
	"github.com/gauas/upload-service/infra"
)

// StreamUploadMessage represents the message structure from the queue
type StreamUploadMessage struct {
	UploadType   string            `json:"upload_type"`   // e.g., "zip", "video", "archive"
	TempBucket   string            `json:"temp_bucket"`   // Bucket in temp MinIO
	TempPath     string            `json:"temp_path"`     // Path in temp MinIO
	TargetBucket string            `json:"target_bucket"` // Target bucket in main MinIO
	TargetFolder string            `json:"target_folder"` // Target folder for chunks
	OriginalName string            `json:"original_name"` // Original file name before hashing
	FileHash     string            `json:"file_hash"`     // Hash of the file
	FileSize     int64             `json:"file_size"`     // Total file size in bytes
	ChunkSize    int64             `json:"chunk_size"`    // Desired chunk size (0 = use default)
	IsHash       bool              `json:"is_hash"`       // Whether to use hash as filename (default: true)
	Metadata     map[string]string `json:"metadata"`      // Additional metadata (user_id, upload_id, etc.)
}

// StreamUploadHandler handles chunked upload messages
type StreamUploadHandler struct {
	infra          *infra.Infra
	chunkerService *service.ChunkerService
}

// NewStreamUploadHandler creates a new chunked upload handler
func NewStreamUploadHandler(infra *infra.Infra, chunkerService *service.ChunkerService) *StreamUploadHandler {
	return &StreamUploadHandler{
		infra:          infra,
		chunkerService: chunkerService,
	}
}

// HandleStreamUpload processes a chunked upload message
func (h *StreamUploadHandler) HandleStreamUpload(ctx context.Context, body []byte) error {
	startTime := time.Now()

	// Parse message
	var msg StreamUploadMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	// Validate message
	if err := h.validateMessage(&msg); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	log.Printf("[StreamUpload] Processing file: %s (hash: %s, size: %d bytes)",
		msg.OriginalName, msg.FileHash, msg.FileSize)

	// Process the file using chunker service
	request := service.ChunkRequest{
		TempBucket:   msg.TempBucket,
		TempPath:     msg.TempPath,
		TargetBucket: msg.TargetBucket,
		TargetFolder: msg.TargetFolder,
		OriginalName: msg.OriginalName,
		FileHash:     msg.FileHash,
		FileSize:     msg.FileSize,
		ChunkSize:    msg.ChunkSize,
		IsHash:       msg.IsHash,
		Metadata:     msg.Metadata,
	}

	result, err := h.chunkerService.ProcessFile(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to process file: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Printf("[StreamUpload] Completed processing %s: uploaded to %s in %v",
		msg.OriginalName, result.FilePath, elapsed)

	return nil
}

// validateMessage validates the chunked upload message
func (h *StreamUploadHandler) validateMessage(msg *StreamUploadMessage) error {
	if msg.TempBucket == "" {
		return fmt.Errorf("temp_bucket is required")
	}
	if msg.TempPath == "" {
		return fmt.Errorf("temp_path is required")
	}
	if msg.TargetBucket == "" {
		return fmt.Errorf("target_bucket is required")
	}
	if msg.FileHash == "" {
		return fmt.Errorf("file_hash is required")
	}
	if msg.FileSize <= 0 {
		return fmt.Errorf("file_size must be positive")
	}
	return nil
}
