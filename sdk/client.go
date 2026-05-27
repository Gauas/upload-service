package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const defaultBasePath = "/v1/upload"

type Config struct {
	BaseURL    string
	SecretKey  string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	secretKey  string
	httpClient *http.Client
}

type UploadRequest struct {
	Reader   io.Reader
	Filename string
	Bucket   string
	Path     string
	IsHash   *bool
}

type UploadResponse struct {
	URL         string `json:"url"`
	FilePath    string `json:"file_path"`
	FileHash    string `json:"file_hash"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Duplicated  bool   `json:"duplicated"`
}

type ListFilesResponse struct {
	Files  []string `json:"files"`
	Count  int      `json:"count"`
	Bucket string   `json:"bucket"`
	Prefix string   `json:"prefix"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type GetFileResponse struct {
	Body        []byte
	ContentType string
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("upload sdk: request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("upload sdk: %s", e.Message)
}

type envelope struct {
	Status int             `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error"`
}

func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, errors.New("upload sdk: base url is required")
	}

	secret := strings.TrimSpace(cfg.SecretKey)
	if secret == "" {
		return nil, errors.New("upload sdk: secret key is required")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("upload sdk: invalid base url: %w", err)
	}

	u.Path = strings.TrimRight(u.Path, "/")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{baseURL: u.String(), secretKey: secret, httpClient: httpClient}, nil
}

func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	endpoint := c.apiURL("health")
	var out HealthResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UploadFile(ctx context.Context, req UploadRequest) (*UploadResponse, error) {
	if req.Reader == nil {
		return nil, errors.New("upload sdk: upload reader is required")
	}
	if strings.TrimSpace(req.Filename) == "" {
		return nil, errors.New("upload sdk: filename is required")
	}
	if strings.TrimSpace(req.Bucket) == "" {
		return nil, errors.New("upload sdk: bucket is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", req.Filename)
	if err != nil {
		return nil, fmt.Errorf("upload sdk: create form file: %w", err)
	}
	if _, err = io.Copy(part, req.Reader); err != nil {
		return nil, fmt.Errorf("upload sdk: write file payload: %w", err)
	}

	if err = writer.WriteField("bucket", req.Bucket); err != nil {
		return nil, fmt.Errorf("upload sdk: write bucket field: %w", err)
	}
	if p := strings.TrimSpace(req.Path); p != "" {
		if err = writer.WriteField("path", p); err != nil {
			return nil, fmt.Errorf("upload sdk: write path field: %w", err)
		}
	}
	if req.IsHash != nil {
		if err = writer.WriteField("is_hash", strconv.FormatBool(*req.IsHash)); err != nil {
			return nil, fmt.Errorf("upload sdk: write is_hash field: %w", err)
		}
	}
	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("upload sdk: close multipart writer: %w", err)
	}

	endpoint := c.apiURL("")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("upload sdk: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Secret-Key", c.secretKey)

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upload sdk: send request: %w", err)
	}
	defer httpRes.Body.Close()

	payload, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, fmt.Errorf("upload sdk: read response body: %w", err)
	}

	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		return nil, c.decodeAPIError(httpRes.StatusCode, payload)
	}

	var env envelope
	if err = json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("upload sdk: decode response envelope: %w", err)
	}

	if env.Error != "" {
		return nil, &APIError{StatusCode: env.Status, Message: env.Error}
	}

	var out UploadResponse
	if err = json.Unmarshal(env.Data, &out); err != nil {
		return nil, fmt.Errorf("upload sdk: decode upload data: %w", err)
	}

	return &out, nil
}

func (c *Client) GetFile(ctx context.Context, bucket, filePath string) (*GetFileResponse, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("upload sdk: bucket is required")
	}
	if strings.TrimSpace(filePath) == "" {
		return nil, errors.New("upload sdk: file path is required")
	}

	q := url.Values{}
	q.Set("bucket", bucket)
	q.Set("file_path", filePath)

	endpoint := c.apiURL("") + "?" + q.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("upload sdk: build request: %w", err)
	}
	httpReq.Header.Set("Secret-Key", c.secretKey)

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upload sdk: send request: %w", err)
	}
	defer httpRes.Body.Close()

	payload, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, fmt.Errorf("upload sdk: read response body: %w", err)
	}

	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		return nil, c.decodeAPIError(httpRes.StatusCode, payload)
	}

	return &GetFileResponse{Body: payload, ContentType: httpRes.Header.Get("Content-Type")}, nil
}

func (c *Client) DeleteFile(ctx context.Context, bucket, filePath string) error {
	if strings.TrimSpace(bucket) == "" {
		return errors.New("upload sdk: bucket is required")
	}
	if strings.TrimSpace(filePath) == "" {
		return errors.New("upload sdk: file path is required")
	}

	q := url.Values{}
	q.Set("bucket", bucket)
	q.Set("file_path", filePath)

	endpoint := c.apiURL("") + "?" + q.Encode()
	return c.doJSON(ctx, http.MethodDelete, endpoint, nil, nil)
}

func (c *Client) ListFiles(ctx context.Context, bucket, prefix string) (*ListFilesResponse, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("upload sdk: bucket is required")
	}

	q := url.Values{}
	q.Set("bucket", bucket)
	if strings.TrimSpace(prefix) != "" {
		q.Set("prefix", prefix)
	}

	endpoint := c.apiURL("list") + "?" + q.Encode()
	var out ListFilesResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, body io.Reader, out interface{}) error {
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("upload sdk: build request: %w", err)
	}
	httpReq.Header.Set("Secret-Key", c.secretKey)

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("upload sdk: send request: %w", err)
	}
	defer httpRes.Body.Close()

	payload, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return fmt.Errorf("upload sdk: read response body: %w", err)
	}

	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		return c.decodeAPIError(httpRes.StatusCode, payload)
	}

	var env envelope
	if err = json.Unmarshal(payload, &env); err != nil {
		return fmt.Errorf("upload sdk: decode response envelope: %w", err)
	}

	if env.Error != "" {
		return &APIError{StatusCode: env.Status, Message: env.Error}
	}

	if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err = json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("upload sdk: decode response data: %w", err)
		}
	}

	return nil
}

func (c *Client) decodeAPIError(statusCode int, payload []byte) error {
	var env envelope
	if err := json.Unmarshal(payload, &env); err == nil {
		if env.Error != "" {
			if env.Status == 0 {
				env.Status = statusCode
			}
			return &APIError{StatusCode: env.Status, Message: env.Error}
		}
	}

	msg := strings.TrimSpace(string(payload))
	return &APIError{StatusCode: statusCode, Message: msg}
}

func (c *Client) apiURL(suffix string) string {
	base, _ := url.Parse(c.baseURL)
	base.Path = path.Join(base.Path, defaultBasePath, suffix)
	return base.String()
}
