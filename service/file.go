package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gauas/upload-service/model"
	"github.com/gauas/upload-service/packages/response"
	"github.com/gauas/upload-service/supports"
)

type UploadParams struct {
	Reader      io.Reader
	Size        int64
	Filename    string
	ContentType string
	Bucket      string
	Path        string
	IsHash      bool
}

type UploadResult struct {
	URL         string `json:"url"`
	FilePath    string `json:"file_path"`
	FileHash    string `json:"file_hash"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Duplicated  bool   `json:"duplicated"`
}

func (s *Service) Upload(ctx context.Context, p UploadParams) (*UploadResult, error) {
	if p.Size > s.cfg.FileMaxSize {
		return nil, response.NewError(400, fmt.Sprintf("file size %d exceeds limit %d", p.Size, s.cfg.FileMaxSize))
	}

	tempFile, cleanup, err := s.writeTempFile(p.Reader)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	fileHash, err := hashFile(tempFile)
	if err != nil {
		return nil, err
	}

	if p.ContentType == "" {
		p.ContentType, err = detectContentType(tempFile)
		if err != nil {
			return nil, err
		}
	}

	ext := extensionFrom(p.Filename, p.ContentType)
	fileName := buildFileName(p.IsHash, fileHash, p.Filename, ext)
	customPath := normalizePath(p.Path)
	fullPath := buildFullPath(customPath, fileName)

	existingPath, exists, err := s.infra.Metadata.CheckByHash(ctx, p.Bucket, fileHash)
	if err != nil {
		return nil, fmt.Errorf("service: check hash: %w", err)
	}
	if exists && existingPath == fullPath {
		return &UploadResult{
			URL:         supports.JoinURL(s.cfg.CDNURL, existingPath),
			FilePath:    existingPath,
			FileHash:    fileHash,
			ContentType: p.ContentType,
			Size:        p.Size,
			Duplicated:  true,
		}, nil
	}

	if customPath != "" && p.Bucket != "pending" {
		for _, seg := range pathSegments(customPath) {
			_ = s.infra.Storage.EnsureFolder(ctx, p.Bucket, seg)
		}
	}

	if _, err := tempFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("service: seek temp file: %w", err)
	}

	meta := map[string]string{
		"file-hash":     fileHash,
		"original-name": p.Filename,
		"content-type":  p.ContentType,
	}
	if err := s.infra.Storage.Put(ctx, p.Bucket, fullPath, tempFile, p.Size, p.ContentType, meta); err != nil {
		return nil, fmt.Errorf("service: upload: %w", err)
	}

	_ = s.infra.Metadata.Add(ctx, model.FileMetadata{
		FileHash:     fileHash,
		FilePath:     fullPath,
		BucketName:   p.Bucket,
		OriginalName: p.Filename,
		ContentType:  p.ContentType,
		FileSize:     p.Size,
		UploadedAt:   time.Now(),
	})

	return &UploadResult{
		URL:         supports.JoinURL(s.cfg.CDNURL, p.Bucket, fullPath),
		FilePath:    fullPath,
		FileHash:    fileHash,
		ContentType: p.ContentType,
		Size:        p.Size,
		Duplicated:  exists && existingPath != fullPath,
	}, nil
}

func (s *Service) Get(ctx context.Context, bucket, path string) ([]byte, string, error) {
	return s.infra.Storage.Get(ctx, bucket, path)
}

func (s *Service) Delete(ctx context.Context, bucket, path string) error {
	if err := s.infra.Storage.Delete(ctx, bucket, path); err != nil {
		return err
	}
	_ = s.infra.Metadata.Remove(ctx, bucket, path)
	return nil
}

func (s *Service) List(ctx context.Context, bucket, prefix string) ([]string, error) {
	return s.infra.Storage.List(ctx, bucket, prefix)
}
