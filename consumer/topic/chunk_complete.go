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
func NewChunkCompleteHandler(i *infra.Infra) *ChunkCompleteHandler {
	return &ChunkCompleteHandler{infra: i}
}

// HandleChunkComplete processes a chunk_complete message
// 1. List and sort chunks from pending bucket
// 2. Stream-compose chunks into a single file with hash calculation
// 3. Upload composed file to target bucket
// 4. Delete chunks from pending bucket
// 5. Publish compose_completed message back to cloud-orchestrator
func (h *ChunkCompleteHandler) HandleChunkComplete(ctx context.Context, body []byte) error {
	startTime := time.Now()

	var msg ChunkCompleteMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("chunk_complete: parse: %w", err)
	}

	log.Printf("[ChunkComplete] upload=%s file=%s chunks=%d target=%s/%s",
		msg.UploadID, msg.FileName, msg.TotalChunks, msg.TargetBucket, msg.TargetPath)

	fileHash, fileSize, filePath, err := h.composeAndUpload(ctx, &msg)

	response := ComposeCompletedMessage{
		UploadID:    msg.UploadID,
		BucketID:    msg.BucketID,
		UserID:      msg.UserID,
		FileHash:    fileHash,
		FilePath:    filePath,
		FileSize:    fileSize,
		ContentType: msg.ContentType,
		FileName:    msg.FileName,
		CustomPath:  msg.CustomPath,
		Success:     err == nil,
		Timestamp:   time.Now().Unix(),
	}
	if err != nil {
		response.Error = err.Error()
		log.Printf("[ChunkComplete] ERROR upload=%s: %v", msg.UploadID, err)
	} else {
		log.Printf("[ChunkComplete] OK upload=%s path=%s hash=%s size=%d elapsed=%v",
			msg.UploadID, filePath, fileHash, fileSize, time.Since(startTime))
	}

	if pubErr := h.publishComposeCompleted(response); pubErr != nil {
		return fmt.Errorf("chunk_complete: publish: %w", pubErr)
	}
	return nil
}

func (h *ChunkCompleteHandler) composeAndUpload(ctx context.Context, msg *ChunkCompleteMessage) (fileHash string, fileSize int64, finalPath string, err error) {
	// 1. List chunks
	allKeys, err := h.infra.Storage.List(ctx, msg.TempBucket, msg.TempPrefix)
	if err != nil {
		return "", 0, "", fmt.Errorf("list chunks: %w", err)
	}

	var chunks []string
	for _, k := range allKeys {
		if strings.HasSuffix(k, "/") {
			continue
		}
		if strings.HasSuffix(k, ".part") {
			chunks = append(chunks, k)
		}
	}

	if len(chunks) == 0 {
		return "", 0, "", fmt.Errorf("no chunks in %s/%s", msg.TempBucket, msg.TempPrefix)
	}
	if len(chunks) != msg.TotalChunks {
		return "", 0, "", fmt.Errorf("chunk mismatch: want %d got %d", msg.TotalChunks, len(chunks))
	}

	sort.Strings(chunks)

	// 2. Stream-compose via pipe
	pipeReader, pipeWriter := io.Pipe()
	hasher := sha256.New()

	type streamResult struct {
		size int64
		err  error
	}
	resultCh := make(chan streamResult, 1)

	go func() {
		defer pipeWriter.Close()
		var total int64
		for i, key := range chunks {
			rc, _, sErr := h.infra.Storage.GetStream(ctx, msg.TempBucket, key)
			if sErr != nil {
				resultCh <- streamResult{0, fmt.Errorf("get chunk %d: %w", i, sErr)}
				return
			}
			w := io.MultiWriter(pipeWriter, hasher)
			n, sErr := io.Copy(w, rc)
			rc.Close()
			if sErr != nil {
				resultCh <- streamResult{0, fmt.Errorf("stream chunk %d: %w", i, sErr)}
				return
			}
			total += n
			log.Printf("[ChunkComplete] streamed chunk %d/%d (%d bytes)", i+1, len(chunks), n)
		}
		resultCh <- streamResult{total, nil}
	}()

	// 3. Upload composed stream to temp key
	ext := filepath.Ext(msg.FileName)
	if ext == "" {
		ext = ".bin"
	}
	tempKey := fmt.Sprintf("_temp_compose/%s%s", msg.UploadID, ext)

	meta := map[string]string{
		"original-name": msg.FileName,
		"content-type":  msg.ContentType,
		"upload-id":     msg.UploadID,
	}

	if putErr := h.infra.Storage.Put(ctx, msg.TargetBucket, tempKey, pipeReader, msg.FileSize, msg.ContentType, meta); putErr != nil {
		pipeReader.Close()
		return "", 0, "", fmt.Errorf("upload composed: %w", putErr)
	}

	res := <-resultCh
	if res.err != nil {
		_ = h.infra.Storage.Delete(ctx, msg.TargetBucket, tempKey)
		return "", 0, "", res.err
	}

	// 4. Build final path and copy temp → final
	fileHash = hex.EncodeToString(hasher.Sum(nil))
	if msg.CustomPath != "" {
		finalPath = fmt.Sprintf("%s/%s", msg.CustomPath, msg.FileName)
	} else {
		finalPath = msg.FileName
	}

	if cpErr := h.infra.Storage.Copy(ctx, msg.TargetBucket, tempKey, msg.TargetBucket, finalPath); cpErr != nil {
		_ = h.infra.Storage.Delete(ctx, msg.TargetBucket, tempKey)
		return "", 0, "", fmt.Errorf("move to final: %w", cpErr)
	}
	_ = h.infra.Storage.Delete(ctx, msg.TargetBucket, tempKey)

	// 5. Async cleanup chunks
	go func() {
		bkg := context.Background()
		for _, k := range chunks {
			if delErr := h.infra.Storage.Delete(bkg, msg.TempBucket, k); delErr != nil {
				log.Printf("[ChunkComplete] warn: delete chunk %s: %v", k, delErr)
			}
		}
		log.Printf("[ChunkComplete] cleaned %d chunks from %s/%s", len(chunks), msg.TempBucket, msg.TempPrefix)
	}()

	return fileHash, res.size, finalPath, nil
}

func (h *ChunkCompleteHandler) publishComposeCompleted(msg ComposeCompletedMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return h.infra.Queue.Publish("upload.exchange", "upload.compose_completed", body)
}
