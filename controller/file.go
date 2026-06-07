package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gauas/upload-service/infra"
	"github.com/gauas/upload-service/utils"
)

func (ctrl *Controller) UploadFile(c *gin.Context) {
	ctx := c.Request.Context()
	ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Upload File] Upload request received")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to get file from form data")
		utils.JSON400(c, "Failed to get file: "+err.Error())
		return
	}

	bucketName := strings.TrimSpace(c.PostForm("bucket"))
	if bucketName == "" {
		ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[Upload File] bucket is required")
		utils.JSON400(c, "bucket parameter is required")
		return
	}

	customPath := strings.TrimSpace(c.PostForm("path"))
	if customPath != "" {
		customPath = strings.Trim(customPath, "/\\")
		customPath = strings.ReplaceAll(customPath, "\\", "/")
		for strings.Contains(customPath, "//") {
			customPath = strings.ReplaceAll(customPath, "//", "/")
		}
		if strings.Contains(customPath, "..") {
			ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[Upload File] Invalid path contains ..")
			utils.JSON400(c, "Invalid path: path cannot contain '..'")
			return
		}
	}

	isHashStr := strings.ToLower(strings.TrimSpace(c.PostForm("is_hash")))
	isHash := true
	if isHashStr == "false" || isHashStr == "0" {
		isHash = false
	}

	maxUploadSize := ctrl.Config.Limit.FileMaxSize
	if fileHeader.Size > maxUploadSize {
		ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[Upload File] File size exceeds limit: %d bytes", fileHeader.Size)
		utils.JSON400(c, fmt.Sprintf("File size exceeds %d bytes limit", maxUploadSize))
		return
	}

	srcFile, err := fileHeader.Open()
	if err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to open uploaded file")
		utils.JSON500(c, "Failed to open file: "+err.Error())
		return
	}
	defer srcFile.Close()

	tempDir := ctrl.Config.Chunk.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to create temp dir")
		utils.JSON500(c, "Failed to create temp dir: "+err.Error())
		return
	}

	tempFile, err := os.CreateTemp(tempDir, "upload-*")
	if err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to create temp file")
		utils.JSON500(c, "Failed to create temp file: "+err.Error())
		return
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(writer, srcFile); err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to stream file to temp storage")
		utils.JSON500(c, "Failed to stream file: "+err.Error())
		return
	}

	fileHash := hex.EncodeToString(hasher.Sum(nil))
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		if _, err := tempFile.Seek(0, 0); err != nil {
			ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to seek temp file")
			utils.JSON500(c, "Failed to seek file: "+err.Error())
			return
		}
		buffer := make([]byte, 512)
		n, _ := tempFile.Read(buffer)
		contentType = http.DetectContentType(buffer[:n])
	}

	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = getExtensionFromContentType(contentType)
	}

	var fileName string
	if isHash {
		fileName = fileHash + ext
	} else {
		fileName = fileHeader.Filename
	}

	var fullPath string
	if customPath != "" {
		fullPath = fmt.Sprintf("%s/%s", customPath, fileName)
		ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Upload File] Upload to path: %s", fullPath)
	} else {
		fullPath = fileName
		ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Upload File] Upload to root: %s", fullPath)
	}

	if customPath != "" && bucketName != "pending" {
		segments := strings.Split(customPath, "/")
		for i := 0; i < len(segments); i++ {
			folder := strings.Join(segments[:i+1], "/")
			if err := ctrl.Infrastructure.MinioClient.CreateFolderIfNotExist(ctx, bucketName, folder); err != nil {
				ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[Upload File] Warning: Failed to create folder %s: %v", folder, err)
			}
		}
	}

	existingFile, exists, err := ctrl.Infrastructure.ParquetService.CheckFileByHash(ctx, bucketName, fileHash)
	if err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to check file existence")
		utils.JSON500(c, "Failed to check file existence: "+err.Error())
		return
	}

	if exists {
		if existingFile == fullPath {
			ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Upload File] File already exists at exact path: %s (hash: %s)", existingFile, fileHash)
			utils.JSON200(c, gin.H{
				"file_path":    existingFile,
				"file_hash":    fileHash,
				"message":      "File already exists (deduplicated)",
				"bucket":       bucketName,
				"content_type": contentType,
				"size":         fileHeader.Size,
				"duplicated":   true,
			})
			return
		}
		ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Upload File] File with same hash exists at %s, but creating copy at new path: %s", existingFile, fullPath)
	}

	if _, err := tempFile.Seek(0, 0); err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to seek temp file for upload")
		utils.JSON500(c, "Failed to prepare file for upload: "+err.Error())
		return
	}

	metadata := map[string]string{
		"file-hash":     fileHash,
		"original-name": fileHeader.Filename,
		"content-type":  contentType,
	}

	if err := ctrl.Infrastructure.MinioClient.PutObjectStreamWithMetadata(ctx, bucketName, fullPath, tempFile, fileHeader.Size, contentType, metadata); err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to upload file to MinIO")
		utils.JSON500(c, "Failed to upload file: "+err.Error())
		return
	}

	fileMetadata := infra.FileMetadata{
		FileHash:     fileHash,
		FilePath:     fullPath,
		BucketName:   bucketName,
		OriginalName: fileHeader.Filename,
		ContentType:  contentType,
		FileSize:     fileHeader.Size,
		UploadedAt:   time.Now(),
	}
	if err := ctrl.Infrastructure.ParquetService.AddFileMetadata(ctx, fileMetadata); err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Upload File] Failed to save metadata to Parquet")
	}

	ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Upload File] File uploaded successfully: %s (hash: %s)", fullPath, fileHash)
	utils.JSON200(c, gin.H{
		"file_path":    fullPath,
		"file_hash":    fileHash,
		"message":      "File uploaded successfully",
		"bucket":       bucketName,
		"content_type": contentType,
		"size":         fileHeader.Size,
		"duplicated":   exists && existingFile != fullPath,
	})
}

func (ctrl *Controller) GetFile(c *gin.Context) {
	ctx := c.Request.Context()
	filePath := c.Query("file_path")
	bucketName := c.Query("bucket")

	ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Get File] Request received - Bucket: %s, Path: %s", bucketName, filePath)

	if filePath == "" {
		ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[Get File] file_path is required")
		utils.JSON400(c, "file_path is required")
		return
	}
	if bucketName == "" {
		ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[Get File] bucket is required")
		utils.JSON400(c, "bucket parameter is required")
		return
	}

	data, contentType, err := ctrl.Infrastructure.MinioClient.GetObjectFromBucket(ctx, bucketName, filePath)
	if err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Get File] Failed to get file from MinIO - Bucket: %s, Path: %s, Error: %v", bucketName, filePath, err)
		utils.JSON404(c, "File not found: "+err.Error())
		return
	}

	ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Get File] File retrieved successfully - Bucket: %s, Path: %s, ContentType: %s, Size: %d bytes", bucketName, filePath, contentType, len(data))
	c.Data(http.StatusOK, contentType, data)
}

func (ctrl *Controller) DeleteFile(c *gin.Context) {
	ctx := c.Request.Context()
	filePath := c.Query("file_path")
	bucketName := c.Query("bucket")

	if filePath == "" {
		ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[Delete File] file_path is required")
		utils.JSON400(c, "file_path is required")
		return
	}
	if bucketName == "" {
		ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[Delete File] bucket is required")
		utils.JSON400(c, "bucket parameter is required")
		return
	}

	if err := ctrl.Infrastructure.MinioClient.DeleteObjectFromBucket(ctx, bucketName, filePath); err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Delete File] Failed to delete file from MinIO")
		utils.JSON500(c, "Failed to delete file: "+err.Error())
		return
	}

	if err := ctrl.Infrastructure.ParquetService.RemoveFileMetadata(ctx, bucketName, filePath); err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[Delete File] Failed to remove metadata from Parquet")
	}

	ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[Delete File] File deleted successfully: %s", filePath)
	utils.JSON200(c, gin.H{"file_path": filePath, "bucket": bucketName, "message": "File deleted successfully"})
}

func (ctrl *Controller) ListFiles(c *gin.Context) {
	ctx := c.Request.Context()
	prefix := c.Query("prefix")
	bucketName := c.Query("bucket")

	if bucketName == "" {
		ctrl.Provider.LoggerProvider.WarningWithContextf(ctx, "[List Files] bucket is required")
		utils.JSON400(c, "bucket parameter is required")
		return
	}

	files, err := ctrl.Infrastructure.MinioClient.ListObjectsFromBucket(ctx, bucketName, prefix)
	if err != nil {
		ctrl.Provider.LoggerProvider.ErrorWithContextf(ctx, err, "[List Files] Failed to list files from MinIO")
		utils.JSON500(c, "Failed to list files: "+err.Error())
		return
	}

	ctrl.Provider.LoggerProvider.InfoWithContextf(ctx, "[List Files] Listed %d files with prefix: %s in bucket: %s", len(files), prefix, bucketName)
	utils.JSON200(c, gin.H{"files": files, "count": len(files), "bucket": bucketName, "prefix": prefix})
}

func getExtensionFromContentType(contentType string) string {
	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	case "application/json":
		return ".json"
	case "text/plain":
		return ".txt"
	case "text/html":
		return ".html"
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg":
		return ".mp3"
	default:
		return ".bin"
	}
}
