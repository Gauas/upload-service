package metadata

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/gauas/upload-service/model"
	"github.com/gauas/upload-service/packages/storage"
	"github.com/parquet-go/parquet-go"
)

const (
	bucket = "metadata"
	file   = "files-metadata.parquet"
)

type Service struct {
	store *storage.Client
}

func New(store *storage.Client) *Service {
	return &Service{store: store}
}

func (s *Service) CheckByHash(ctx context.Context, targetBucket, hash string) (string, bool, error) {
	records, err := s.load(ctx)
	if err != nil {
		return "", false, err
	}
	for _, r := range records {
		if r.FileHash == hash && r.BucketName == targetBucket {
			return r.FilePath, true, nil
		}
	}
	return "", false, nil
}

func (s *Service) Add(ctx context.Context, meta model.FileMetadata) error {
	records, err := s.load(ctx)
	if err != nil {
		return err
	}
	for i, r := range records {
		if r.FileHash == meta.FileHash && r.BucketName == meta.BucketName {
			records[i] = meta
			return s.save(ctx, records)
		}
	}
	records = append(records, meta)
	return s.save(ctx, records)
}

func (s *Service) Remove(ctx context.Context, targetBucket, filePath string) error {
	records, err := s.load(ctx)
	if err != nil {
		return err
	}
	filtered := records[:0]
	for _, r := range records {
		if !(r.FilePath == filePath && r.BucketName == targetBucket) {
			filtered = append(filtered, r)
		}
	}
	return s.save(ctx, filtered)
}

func (s *Service) load(ctx context.Context) ([]model.FileMetadata, error) {
	data, _, err := s.store.Get(ctx, bucket, file)
	if err != nil {
		return []model.FileMetadata{}, nil
	}

	r := parquet.NewGenericReader[model.FileMetadata](bytes.NewReader(data))
	defer r.Close()

	var records []model.FileMetadata
	buf := make([]model.FileMetadata, 1000)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			records = append(records, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("metadata: read parquet: %w", err)
		}
	}
	return records, nil
}

func (s *Service) save(ctx context.Context, records []model.FileMetadata) error {
	buf := new(bytes.Buffer)
	w := parquet.NewGenericWriter[model.FileMetadata](buf, parquet.Compression(&parquet.Snappy))

	if _, err := w.Write(records); err != nil {
		return fmt.Errorf("metadata: write parquet: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("metadata: close parquet writer: %w", err)
	}
	if err := s.store.PutRaw(ctx, bucket, file, buf.Bytes(), "application/octet-stream"); err != nil {
		return fmt.Errorf("metadata: upload parquet: %w", err)
	}
	return nil
}
