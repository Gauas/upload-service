package infra

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appconfig "github.com/gauas/upload-service/config"
)

type MinioClient struct {
	Client *s3.Client
}

func NewMinioClient(cfg *appconfig.Config) (*MinioClient, error) {
	endpoint := cfg.Storage.Endpoint
	accessKey := cfg.Storage.AccessKey
	secretKey := cfg.Storage.SecretKey
	region := cfg.Storage.Region
	useSSL := cfg.Storage.UseSSL

	if region == "" {
		region = "us-east-1"
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, reg string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpoint,
			SigningRegion:     region,
			HostnameImmutable: true,
		}, nil
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for MinIO: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		if !useSSL {
			o.EndpointOptions.DisableHTTPS = true
		}
	})

	return &MinioClient{Client: s3Client}, nil
}

func (m *MinioClient) PutObjectWithMetadata(ctx context.Context, bucket, key string, data []byte, contentType string, metadata map[string]string) error {
	if err := m.EnsureBucketByName(ctx, bucket); err != nil {
		return err
	}

	_, err := m.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to put object with metadata: %w", err)
	}
	return nil
}

func (m *MinioClient) PutObjectStreamWithMetadata(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string, metadata map[string]string) error {
	if err := m.EnsureBucketByName(ctx, bucket); err != nil {
		return err
	}

	uploader := manager.NewUploader(m.Client, func(u *manager.Uploader) {
		u.PartSize = 5 * 1024 * 1024
		u.Concurrency = 1
	})

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to put object stream with metadata: %w", err)
	}
	return nil
}

func (m *MinioClient) CheckFileExistsByHash(ctx context.Context, bucket, hash string) (string, bool, error) {
	resp, err := m.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	if err != nil {
		return "", false, fmt.Errorf("failed to list objects: %w", err)
	}

	for _, item := range resp.Contents {
		headResp, err := m.Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: item.Key})
		if err != nil {
			continue
		}
		if fileHash, ok := headResp.Metadata["file-hash"]; ok && fileHash == hash {
			return aws.ToString(item.Key), true, nil
		}
	}

	return "", false, nil
}

func (m *MinioClient) GetObjectFromBucket(ctx context.Context, bucket, key string) ([]byte, string, error) {
	resp, err := m.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), aws.ToString(resp.ContentType), nil
}

func (m *MinioClient) GetObjectStream(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	resp, err := m.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get object stream: %w", err)
	}

	size := int64(0)
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}

	return resp.Body, size, nil
}

func (m *MinioClient) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := m.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

func (m *MinioClient) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	copySource := fmt.Sprintf("%s/%s", srcBucket, srcKey)
	_, err := m.Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}
	return nil
}

func (m *MinioClient) DeleteObjectFromBucket(ctx context.Context, bucket, key string) error {
	_, err := m.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

func (m *MinioClient) ListObjectsFromBucket(ctx context.Context, bucket, prefix string) ([]string, error) {
	resp, err := m.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix)})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	var keys []string
	for _, item := range resp.Contents {
		keys = append(keys, aws.ToString(item.Key))
	}
	return keys, nil
}

func (m *MinioClient) EnsureBucketByName(ctx context.Context, bucket string) error {
	_, err := m.Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		_, err = m.Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	return nil
}

func (m *MinioClient) CreateFolderIfNotExist(ctx context.Context, bucket, folderPath string) error {
	if err := m.EnsureBucketByName(ctx, bucket); err != nil {
		return err
	}

	if !strings.HasSuffix(folderPath, "/") {
		folderPath += "/"
	}

	_, err := m.Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(folderPath)})
	if err == nil {
		return nil
	}

	_, err = m.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(folderPath),
		Body:          bytes.NewReader([]byte{}),
		ContentLength: aws.Int64(0),
		ContentType:   aws.String("application/x-directory"),
	})
	if err != nil {
		return fmt.Errorf("failed to create folder marker: %w", err)
	}

	return nil
}
