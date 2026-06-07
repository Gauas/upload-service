package topic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gauas/upload-service/infra"
)

// ChunkCompleteMessage is received from cloud-orchestrator when all chunks are uploaded
type ChunkCompleteMessage struct {
	UploadID     string            `json:"upload_id"`
	BucketID     string            `json:"bucket_id"`
	BucketName   string            `json:"bucket_name"`
	UserID       string            `json:"user_id"`
	TempBucket   string            `json:"temp_bucket"`
	TempPrefix   string            `json:"temp_prefix"`
	FileName     string            `json:"file_name"`
	FileSize     int64             `json:"file_size"`
	ContentType  string            `json:"content_type"`
	CustomPath   string            `json:"custom_path"`
	TotalChunks  int               `json:"total_chunks"`
	TargetBucket string            `json:"target_bucket"`
	TargetPath   string            `json:"target_path"`
	Metadata     map[string]string `json:"metadata"`
	Timestamp    int64             `json:"timestamp"`
}

// ComposeCompletedMessage is sent back to cloud-orchestrator after compose is done
type ComposeCompletedMessage struct {
	UploadID    string `json:"upload_id"`
	BucketID    string `json:"bucket_id"`
	UserID      string `json:"user_id"`
	FileHash    string `json:"file_hash"`
	FilePath    string `json:"file_path"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	FileName    string `json:"file_name"`
	CustomPath  string `json:"custom_path"`
	Success     bool   `json:"success"`
	Error       string `json:"error"`
	Timestamp   int64  `json:"timestamp"`
}

// ChunkCompleteHandler handles chunk_complete messages from cloud-orchestrator
type ChunkCompleteHandler struct {
	infra *infra.Infra
}

// NewChunkCompleteHandler creates a new chunk complete handler
func NewChunkCompleteHandler(infra *infra.Infra) *ChunkCompleteHandler {
	return &ChunkCompleteHandler{
		infra: infra,
	}
}

// HandleChunkComplete processes a chunk_complete message
// 1. List and sort chunks from pending bucket
// 2. Stream compose chunks into a single file with hash calculation
// 3. Upload composed file to target bucket
// 4. Delete chunks from pending bucket
// 5. Send compose_completed message back to cloud-orchestrator
func (h *ChunkCompleteHandler) HandleChunkComplete(ctx context.Context, body []byte) error {
	startTime := time.Now()

	// Parse message
	var msg ChunkCompleteMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("failed to parse chunk_complete message: %w", err)
	}

	log.Printf("[ChunkComplete] Processing upload %s: %s (%d chunks, target: %s/%s)",
		msg.UploadID, msg.FileName, msg.TotalChunks, msg.TargetBucket, msg.TargetPath)

	// Process compose and get result
	fileHash, fileSize, err := h.composeAndUpload(ctx, &msg)

	// Prepare response message
	response := ComposeCompletedMessage{
		UploadID:    msg.UploadID,
		BucketID:    msg.BucketID,
		UserID:      msg.UserID,
		FileHash:    fileHash,
		FileSize:    fileSize,
		ContentType: msg.ContentType,
		FileName:    msg.FileName,
		CustomPath:  msg.CustomPath,
		Success:     err == nil,
		Timestamp:   time.Now().Unix(),
	}

	if err != nil {
		response.Error = err.Error()
		log.Printf("[ChunkComplete] Failed to compose upload %s: %v", msg.UploadID, err)
	} else {
		// Construct final file path using original filename (no hash)
		fileName := msg.FileName
		if msg.CustomPath != "" {
			response.FilePath = fmt.Sprintf("%s/%s", msg.CustomPath, fileName)
		} else {
			response.FilePath = fileName
		}
		log.Printf("[ChunkComplete] Successfully composed upload %s -> %s (hash: %s, size: %d)",
			msg.UploadID, response.FilePath, fileHash, fileSize)
	}

	// Publish compose_completed message back to cloud-orchestrator
	if err := h.publishComposeCompleted(ctx, response); err != nil {
		return fmt.Errorf("failed to publish compose_completed: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Printf("[ChunkComplete] Completed processing upload %s in %v", msg.UploadID, elapsed)

	return nil
}

// composeAndUpload streams chunks, calculates hash, and uploads to target bucket
func (h *ChunkCompleteHandler) composeAndUpload(ctx context.Context, msg *ChunkCompleteMessage) (string, int64, error) {
	// 1. List all chunks from pending bucket
	chunkPrefix := msg.TempPrefix // e.g., "{upload_id}/"
	allObjects, err := h.infra.MinioClient.ListObjectsFromBucket(ctx, msg.TempBucket, chunkPrefix)
	if err != nil {
		return "", 0, fmt.Errorf("failed to list chunks: %w", err)
	}

	// Filter out folder markers and non-chunk files
	// Only keep files ending with .part (actual chunk files)
	var chunks []string
	for _, key := range allObjects {
		// Skip folder markers (keys ending with /)
		if strings.HasSuffix(key, "/") {
			log.Printf("[ChunkComplete] Skipping folder marker: %s", key)
			continue
		}
		// Only include .part files (actual chunks)
		if strings.HasSuffix(key, ".part") {
			chunks = append(chunks, key)
		} else {
			log.Printf("[ChunkComplete] Skipping non-chunk file: %s", key)
		}
	}

	if len(chunks) == 0 {
		return "", 0, fmt.Errorf("no chunks found in %s/%s", msg.TempBucket, chunkPrefix)
	}

	if len(chunks) != msg.TotalChunks {
		return "", 0, fmt.Errorf("chunk count mismatch: expected %d, found %d (total objects: %d)", msg.TotalChunks, len(chunks), len(allObjects))
	}

	// 2. Sort chunks by name (chunk_00000.part, chunk_00001.part, ...)
	sort.Strings(chunks)
	log.Printf("[ChunkComplete] Found %d chunks for upload %s", len(chunks), msg.UploadID)

	// 3. Create a pipe to stream composed file
	// Use io.Pipe for true streaming without loading everything into memory
	pipeReader, pipeWriter := io.Pipe()
	hasher := sha256.New()

	// Channel to capture result from goroutine (size and error)
	type streamResult struct {
		totalSize int64
		err       error
	}
	resultChan := make(chan streamResult, 1)

	// Start streaming chunks in a goroutine
	go func() {
		defer pipeWriter.Close()

		var totalSize int64
		for i, chunkKey := range chunks {
			// Get chunk stream from MinIO
			chunkStream, size, err := h.infra.MinioClient.GetObjectStream(ctx, msg.TempBucket, chunkKey)
			if err != nil {
				resultChan <- streamResult{0, fmt.Errorf("failed to get chunk %d stream: %w", i, err)}
				return
			}

			// Stream chunk to both hasher and pipe writer
			writer := io.MultiWriter(pipeWriter, hasher)
			written, err := io.Copy(writer, chunkStream)
			chunkStream.Close()

			if err != nil {
				resultChan <- streamResult{0, fmt.Errorf("failed to stream chunk %d: %w", i, err)}
				return
			}

			totalSize += written
			log.Printf("[ChunkComplete] Streamed chunk %d/%d (%d bytes, size hint: %d)", i+1, len(chunks), written, size)
		}

		resultChan <- streamResult{totalSize, nil}
	}()

	// 4. Determine final file path
	ext := filepath.Ext(msg.FileName)
	if ext == "" {
		ext = ".bin"
	}

	// We need to upload while streaming, but we don't have the hash yet
	// So we'll upload to a temp location first, then rename after we have the hash
	tempUploadKey := fmt.Sprintf("_temp_compose/%s%s", msg.UploadID, ext)

	// 5. Upload composed stream to target bucket
	// Use the reader from pipe
	metadata := map[string]string{
		"original-name": msg.FileName,
		"content-type":  msg.ContentType,
		"upload-id":     msg.UploadID,
	}

	log.Printf("[ChunkComplete] Uploading composed file to %s/%s", msg.TargetBucket, tempUploadKey)

	if err := h.infra.MinioClient.PutObjectStreamWithMetadata(
		ctx,
		msg.TargetBucket,
		tempUploadKey,
		pipeReader,
		msg.FileSize, // Expected size
		msg.ContentType,
		metadata,
	); err != nil {
		pipeReader.Close()
		return "", 0, fmt.Errorf("failed to upload composed file: %w", err)
	}

	// Wait for streaming goroutine to finish and get result
	result := <-resultChan
	if result.err != nil {
		// Cleanup temp file
		_ = h.infra.MinioClient.DeleteObject(ctx, msg.TargetBucket, tempUploadKey)
		return "", 0, result.err
	}

	totalSize := result.totalSize

	// 6. Calculate final hash
	fileHash := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("[ChunkComplete] Calculated hash: %s (total size: %d)", fileHash, totalSize)

	// 7. Rename/copy temp file to final location with original filename (no hash)
	fileName := msg.FileName
	var finalPath string
	if msg.CustomPath != "" {
		finalPath = fmt.Sprintf("%s/%s", msg.CustomPath, fileName)
	} else {
		finalPath = fileName
	}

	// Copy from temp to final location
	log.Printf("[ChunkComplete] Moving composed file to final location: %s/%s", msg.TargetBucket, finalPath)
	if err := h.infra.MinioClient.CopyObject(ctx, msg.TargetBucket, tempUploadKey, msg.TargetBucket, finalPath); err != nil {
		// Cleanup temp file
		_ = h.infra.MinioClient.DeleteObject(ctx, msg.TargetBucket, tempUploadKey)
		return "", 0, fmt.Errorf("failed to move to final location: %w", err)
	}

	// Delete temp file
	_ = h.infra.MinioClient.DeleteObject(ctx, msg.TargetBucket, tempUploadKey)

	// 8. Cleanup chunks from pending bucket (async)
	go func() {
		cleanupCtx := context.Background()
		for _, chunkKey := range chunks {
			if err := h.infra.MinioClient.DeleteObject(cleanupCtx, msg.TempBucket, chunkKey); err != nil {
				log.Printf("[ChunkComplete] Warning: failed to delete chunk %s: %v", chunkKey, err)
			}
		}
		log.Printf("[ChunkComplete] Cleaned up %d chunks from %s/%s", len(chunks), msg.TempBucket, chunkPrefix)
	}()

	return fileHash, totalSize, nil
}

// publishComposeCompleted sends compose_completed message to cloud-orchestrator
func (h *ChunkCompleteHandler) publishComposeCompleted(ctx context.Context, msg ComposeCompletedMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal compose_completed message: %w", err)
	}

	// Publish to compose_completed queue
	return h.infra.RabbitMQ.PublishToExchange(
		"upload.exchange",
		"upload.compose_completed",
		body,
	)
}
