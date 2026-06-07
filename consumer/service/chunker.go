package service

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/infra"
)

// ChunkRequest represents a request to process a large file
type ChunkRequest struct {
	TempBucket   string
	TempPath     string
	TargetBucket string
	TargetFolder string
	OriginalName string
	FileHash     string
	FileSize     int64
	ChunkSize    int64
	IsHash       bool
	Metadata     map[string]string
}

// ProcessResult represents the result of processing a large file
type ProcessResult struct {
	OriginalFile string            `json:"original_file"`
	FileHash     string            `json:"file_hash"`
	TotalSize    int64             `json:"total_size"`
	FilePath     string            `json:"file_path"`
	ContentType  string            `json:"content_type"`
	CreatedAt    time.Time         `json:"created_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ChunkerService handles large file processing operations
type ChunkerService struct {
	config           *config.Config
	infra            *infra.Infra
	defaultChunkSize int64
}

// NewChunkerService creates a new chunker service
func NewChunkerService(cfg *config.Config, inf *infra.Infra) *ChunkerService {
	return &ChunkerService{
		config:           cfg,
		infra:            inf,
		defaultChunkSize: cfg.Chunk.DefaultChunkSize,
	}
}

// ProcessFile streams a file from temp MinIO directly to main MinIO without disk I/O
// The file has already been merged by the backend, consumer just moves it to the final destination
func (s *ChunkerService) ProcessFile(ctx context.Context, req ChunkRequest) (*ProcessResult, error) {
	log.Printf("[Chunker] Processing file from %s/%s (size: %d bytes)",
		req.TempBucket, req.TempPath, req.FileSize)

	// Determine content type from metadata or default
	contentType := "application/octet-stream"
	if ct, ok := req.Metadata["content_type"]; ok && ct != "" {
		contentType = ct
	}

	// Construct final file path based on is_hash parameter
	ext := filepath.Ext(req.OriginalName)
	if ext == "" {
		ext = filepath.Ext(req.TempPath)
	}

	// Determine filename based on IsHash parameter
	var fileName string
	if req.IsHash {
		// Use hash as filename (default behavior)
		fileName = req.FileHash + ext
	} else {
		// Use original filename
		fileName = req.OriginalName
	}

	var finalPath string
	if req.TargetFolder != "" && req.TargetFolder != req.FileHash {
		// If TargetFolder contains customPath/hash, extract just the customPath
		customPath := req.Metadata["custom_path"]
		if customPath != "" {
			finalPath = fmt.Sprintf("%s/%s", customPath, fileName)
		} else {
			finalPath = fileName
		}
	} else {
		finalPath = fileName
	}

	log.Printf("[Chunker] Streaming file to %s/%s (no disk I/O)", req.TargetBucket, finalPath)

	// Stream file directly from temp MinIO to main MinIO (no disk I/O)
	if err := s.streamToMain(ctx, req.TempBucket, req.TempPath, req.TargetBucket, finalPath, contentType, req); err != nil {
		return nil, fmt.Errorf("failed to stream to main MinIO: %w", err)
	}

	// Clean up temp file in temp MinIO
	if err := s.cleanupTemp(ctx, req.TempBucket, req.TempPath); err != nil {
		log.Printf("[Chunker] Warning: failed to cleanup temp file: %v", err)
		// Don't fail the whole operation for cleanup failure
	}

	result := &ProcessResult{
		OriginalFile: req.OriginalName,
		FileHash:     req.FileHash,
		TotalSize:    req.FileSize,
		FilePath:     finalPath,
		ContentType:  contentType,
		CreatedAt:    time.Now(),
		Metadata:     req.Metadata,
	}

	log.Printf("[Chunker] Successfully processed file: %s -> %s/%s",
		req.OriginalName, req.TargetBucket, finalPath)

	return result, nil
}

// streamToMain streams a file directly from temp MinIO to main MinIO without disk I/O
func (s *ChunkerService) streamToMain(ctx context.Context, tempBucket, tempPath, mainBucket, mainKey, contentType string, req ChunkRequest) error {
	log.Printf("[Chunker] Starting direct stream from %s/%s to %s/%s", tempBucket, tempPath, mainBucket, mainKey)

	// Get object stream from temp MinIO
	stream, size, err := s.infra.MinioClient.GetObjectStream(ctx, tempBucket, tempPath)
	if err != nil {
		return fmt.Errorf("failed to get object stream: %w", err)
	}
	defer stream.Close()

	// Prepare metadata
	metadata := map[string]string{
		"file-hash":     req.FileHash,
		"original-name": req.OriginalName,
		"content-type":  contentType,
	}
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	// Stream directly to main MinIO using PutObjectStreamWithMetadata
	// This method accepts io.Reader and streams without loading into memory
	if err := s.infra.MinioClient.PutObjectStreamWithMetadata(ctx, mainBucket, mainKey, stream, size, contentType, metadata); err != nil {
		return fmt.Errorf("failed to stream object: %w", err)
	}

	log.Printf("[Chunker] Successfully streamed %d bytes to %s/%s", size, mainBucket, mainKey)
	return nil
}

// cleanupTemp deletes the temporary file from temp MinIO
func (s *ChunkerService) cleanupTemp(ctx context.Context, bucket, path string) error {
	if err := s.infra.MinioClient.DeleteObject(ctx, bucket, path); err != nil {
		return fmt.Errorf("failed to delete temp file: %w", err)
	}
	log.Printf("[Chunker] Cleaned up temp file: %s/%s", bucket, path)
	return nil
}
