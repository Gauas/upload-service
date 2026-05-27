package model

import "time"

type FileMetadata struct {
	FileHash     string    `parquet:"file_hash,snappy"`
	FilePath     string    `parquet:"file_path,snappy"`
	BucketName   string    `parquet:"bucket_name,snappy"`
	OriginalName string    `parquet:"original_name,snappy"`
	ContentType  string    `parquet:"content_type,snappy"`
	FileSize     int64     `parquet:"file_size"`
	UploadedAt   time.Time `parquet:"uploaded_at"`
}
