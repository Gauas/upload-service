package infra

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/parquet-go/parquet-go"
)

type FileMetadata struct {
	FileHash     string    `parquet:"file_hash,snappy"`
	FilePath     string    `parquet:"file_path,snappy"`
	BucketName   string    `parquet:"bucket_name,snappy"`
	OriginalName string    `parquet:"original_name,snappy"`
	ContentType  string    `parquet:"content_type,snappy"`
	FileSize     int64     `parquet:"file_size"`
	UploadedAt   time.Time `parquet:"uploaded_at"`
}

type ParquetService struct {
	minioClient    *MinioClient
	metadataBucket string
	metadataFile   string
}

func NewParquetService(minioClient *MinioClient) *ParquetService {
	return &ParquetService{
		minioClient:    minioClient,
		metadataBucket: "metadata",
		metadataFile:   "files-metadata.parquet",
	}
}

func (ps *ParquetService) LoadMetadata(ctx context.Context) ([]FileMetadata, error) {
	if err := ps.minioClient.EnsureBucketByName(ctx, ps.metadataBucket); err != nil {
		return nil, fmt.Errorf("failed to ensure metadata bucket: %w", err)
	}

	data, _, err := ps.minioClient.GetObjectFromBucket(ctx, ps.metadataBucket, ps.metadataFile)
	if err != nil {
		return []FileMetadata{}, nil
	}

	reader := bytes.NewReader(data)
	parquetReader := parquet.NewGenericReader[FileMetadata](reader)
	defer parquetReader.Close()

	var metadata []FileMetadata
	rows := make([]FileMetadata, 1000)
	for {
		n, err := parquetReader.Read(rows)
		if n > 0 {
			metadata = append(metadata, rows[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read parquet: %w", err)
		}
	}

	return metadata, nil
}

func (ps *ParquetService) SaveMetadata(ctx context.Context, metadata []FileMetadata) error {
	if err := ps.minioClient.EnsureBucketByName(ctx, ps.metadataBucket); err != nil {
		return fmt.Errorf("failed to ensure metadata bucket: %w", err)
	}

	buf := new(bytes.Buffer)
	parquetWriter := parquet.NewGenericWriter[FileMetadata](buf, parquet.Compression(&parquet.Snappy))

	_, err := parquetWriter.Write(metadata)
	if err != nil {
		return fmt.Errorf("failed to write parquet: %w", err)
	}

	if err := parquetWriter.Close(); err != nil {
		return fmt.Errorf("failed to close parquet writer: %w", err)
	}

	_, err = ps.minioClient.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(ps.metadataBucket),
		Key:         aws.String(ps.metadataFile),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	return nil
}

func (ps *ParquetService) CheckFileByHash(ctx context.Context, bucket, hash string) (string, bool, error) {
	metadata, err := ps.LoadMetadata(ctx)
	if err != nil {
		return "", false, err
	}

	for _, item := range metadata {
		if item.FileHash == hash && item.BucketName == bucket {
			return item.FilePath, true, nil
		}
	}

	return "", false, nil
}

func (ps *ParquetService) AddFileMetadata(ctx context.Context, meta FileMetadata) error {
	metadata, err := ps.LoadMetadata(ctx)
	if err != nil {
		return err
	}

	for i, item := range metadata {
		if item.FileHash == meta.FileHash && item.BucketName == meta.BucketName {
			metadata[i] = meta
			return ps.SaveMetadata(ctx, metadata)
		}
	}

	metadata = append(metadata, meta)
	return ps.SaveMetadata(ctx, metadata)
}

func (ps *ParquetService) RemoveFileMetadata(ctx context.Context, bucket, filePath string) error {
	metadata, err := ps.LoadMetadata(ctx)
	if err != nil {
		return err
	}

	var newMetadata []FileMetadata
	for _, item := range metadata {
		if !(item.FilePath == filePath && item.BucketName == bucket) {
			newMetadata = append(newMetadata, item)
		}
	}

	return ps.SaveMetadata(ctx, newMetadata)
}
