package storage

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
	"github.com/gauas/upload-service/config"
)

type Client struct {
	s3 *s3.Client
}

func New(cfg config.StorageConfig) (*Client, error) {
	region := cfg.Region

	resolver := aws.EndpointResolverWithOptionsFunc(func(service, reg string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               cfg.Endpoint,
			SigningRegion:     region,
			HostnameImmutable: true,
		}, nil
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: load config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		if !cfg.UseSSL {
			o.EndpointOptions.DisableHTTPS = true
		}
	})

	return &Client{s3: s3Client}, nil
}

func (c *Client) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string, metadata map[string]string) error {
	if err := c.ensureBucket(ctx, bucket); err != nil {
		return err
	}

	uploader := manager.NewUploader(c.s3, func(u *manager.Uploader) {
		u.PartSize = 5 * 1024 * 1024
		u.Concurrency = 1
	})

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("storage: put %s/%s: %w", bucket, key, err)
	}
	return nil
}

func (c *Client) Get(ctx context.Context, bucket, key string) ([]byte, string, error) {
	resp, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("storage: get %s/%s: %w", bucket, key, err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return nil, "", fmt.Errorf("storage: read body: %w", err)
	}
	return buf.Bytes(), aws.ToString(resp.ContentType), nil
}

func (c *Client) GetStream(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	resp, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("storage: get stream %s/%s: %w", bucket, key, err)
	}
	size := int64(0)
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}
	return resp.Body, size, nil
}

func (c *Client) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	copySource := fmt.Sprintf("%s/%s", srcBucket, srcKey)
	_, err := c.s3.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return fmt.Errorf("storage: copy %s/%s → %s/%s: %w", srcBucket, srcKey, dstBucket, dstKey, err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("storage: delete %s/%s: %w", bucket, key, err)
	}
	return nil
}

func (c *Client) List(ctx context.Context, bucket, prefix string) ([]string, error) {
	resp, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: list %s: %w", bucket, err)
	}
	var keys []string
	for _, item := range resp.Contents {
		keys = append(keys, aws.ToString(item.Key))
	}
	return keys, nil
}

func (c *Client) PutRaw(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	if err := c.ensureBucket(ctx, bucket); err != nil {
		return err
	}
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return fmt.Errorf("storage: put raw %s/%s: %w", bucket, key, err)
	}
	return nil
}

func (c *Client) EnsureFolder(ctx context.Context, bucket, folderPath string) error {
	if err := c.ensureBucket(ctx, bucket); err != nil {
		return err
	}
	if !strings.HasSuffix(folderPath, "/") {
		folderPath += "/"
	}
	_, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(folderPath),
	})
	if err == nil {
		return nil
	}
	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(folderPath),
		Body:          bytes.NewReader([]byte{}),
		ContentLength: aws.Int64(0),
		ContentType:   aws.String("application/x-directory"),
	})
	if err != nil {
		return fmt.Errorf("storage: ensure folder %s/%s: %w", bucket, folderPath, err)
	}
	return nil
}

func (c *Client) ensureBucket(ctx context.Context, bucket string) error {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}
	_, err = c.s3.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		return fmt.Errorf("storage: create bucket %s: %w", bucket, err)
	}
	return nil
}
