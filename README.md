# Upload Service by Gau

## Introduction | Giới thiệu

**English:**  
This repository provides an upload service written in Go, designed to handle file uploads including images and documents. It supports uploading to cloud storage (Cloudflare R2) with proper file validation and organization. The service is suitable for microservices architectures and can be deployed using Docker or Kubernetes.

**Tiếng Việt:**  
Repo này cung cấp dịch vụ upload file viết bằng Go, dùng để xử lý upload file bao gồm hình ảnh và tài liệu. Hỗ trợ upload lên cloud storage (Cloudflare R2) với kiểm tra file hợp lệ và tổ chức file. Phù hợp với kiến trúc microservices và có thể triển khai bằng Docker hoặc Kubernetes.

---

## Directory Structure | Cấu trúc thư mục

```
gau-upload-service/
├── Dockerfile
├── entrypoint.sh
├── go.mod
├── go.sum
├── main.go
├── README.md
├── config/
│   ├── env_config.go
│   └── main.go
├── controller/
│   ├── file.go
│   └── main.go
├── deploy/
│   └── k8s/
│       ├── production/
│       │   ├── apply.sh
│       │   ├── apply_envsubst.sh
│       │   ├── kustomization.yaml
│       │   ├── unapply.sh
│       │   ├── base/
│       │   └── template/
│       │       ├── configmap.yaml
│       │       ├── deployment.yaml
│       │       ├── hpa.yaml
│       │       ├── ingress.yaml
│       │       ├── secret.yaml
│       │       └── service.yaml
│       └── staging/
│           ├── apply.sh
│           ├── apply_envsubst.sh
│           ├── kustomization.yaml
│           ├── unapply.sh
│           ├── base/
│           └── template/
│               ├── deployment.yaml
│               ├── hpa.yaml
│               ├── ingress.yaml
│               ├── secret.yaml
│               └── service.yaml
├── infra/
│   ├── logger.go
│   ├── main.go
│   ├── minio.go
│   ├── parquet.go
│   ├── postgres.go
│   └── redis.go
├── middlewares/
│   ├── main.go
│   └── private.go
├── provider/
│   ├── logger.go
│   └── main.go
├── repository/
│   └── main.go
├── routes/
│   └── routes.go
└── utils/
    ├── fileCheck.go
    └── http_response.go
```

### 📑 Directory Description | Mô tả thư mục

| Path                          | Description                                             | Mô tả                                  |
|-------------------------------|---------------------------------------------------------|----------------------------------------|
| `Dockerfile`, `entrypoint.sh` | Docker image build and startup script                   | File build và khởi động Docker         |
| `go.mod`, `go.sum`            | Go module definitions                                   | Định nghĩa module Go                   |
| `config/`                     | Environment loading and configuration logic             | Logic cấu hình và load môi trường      |
| `controller/`                 | HTTP handlers for file upload operations                | Xử lý HTTP cho upload file             |
| `deploy/k8s/`                 | Kubernetes manifests and scripts for staging/production | Manifest và script triển khai trên K8s |
| `infra/`                      | MinIO, PostgreSQL, Redis, Parquet setup                 | Thiết lập MinIO, DB, Redis và Parquet  |
| `middlewares/`                | Authentication and other middleware logic               | Middleware xác thực                    |
| `provider/`                   | Logger and other service providers                      | Logger và các provider khác            |
| `repository/`                 | Data access and business logic                          | Truy cập và xử lý dữ liệu              |
| `routes/`                     | API route definitions                                   | Định nghĩa route                       |
| `utils/`                      | File validation and utility functions                   | Kiểm tra file và hàm tiện ích          |

---

## Features | Tính năng

### 📤 File Upload | Upload File

**English:**
- Support for images (JPEG, PNG, WebP) and various file types
- File size validation with configurable limits
- Automatic file name sanitization using SHA-256 hash
- Organized storage with custom folder paths (supports nested paths like `abc/def`)
- Upload to MinIO/S3-compatible storage
- File deduplication using Parquet metadata storage
- Automatic bucket creation if not exists

**Tiếng Việt:**
- Hỗ trợ hình ảnh (JPEG, PNG, WebP) và nhiều loại file khác
- Kiểm tra kích thước file với giới hạn có thể cấu hình
- Tự động làm sạch tên file bằng SHA-256 hash
- Lưu trữ có tổ chức với đường dẫn thư mục tùy chỉnh (hỗ trợ path lồng nhau như `abc/def`)
- Upload lên MinIO/S3-compatible storage
- Loại bỏ trùng lặp file bằng Parquet metadata
- Tự động tạo bucket nếu chưa tồn tại

### 🔒 Security | Bảo mật

**English:**
- File type validation based on content type
- File size limits to prevent abuse
- Input sanitization for file names and paths
- Path traversal protection (blocks `..` in paths)

**Tiếng Việt:**
- Kiểm tra loại file dựa trên content type
- Giới hạn kích thước file để tránh lạm dụng
- Làm sạch đầu vào cho tên file và đường dẫn
- Bảo vệ path traversal (chặn `..` trong đường dẫn)

---

## API Endpoints | Điểm cuối API

### POST /api/v2/upload/file

**Upload a file with optional path organization**

**Request:**
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@image.jpg" \
  -F "bucket=my-bucket" \
  -F "path=user_avatars/profiles" \
  http://localhost:8080/api/v2/upload/file
```

**Response:**
```json
{
  "file_path": "user_avatars/profiles/abc123def456...hash.jpg",
  "file_hash": "abc123def456...hash",
  "message": "File uploaded successfully",
  "bucket": "my-bucket",
  "content_type": "image/jpeg",
  "size": 102400,
  "duplicated": false
}
```

**Parameters:**
- `file`: The file to upload (required)
- `bucket`: Bucket name where the file will be stored (required)
- `path`: Optional folder path (e.g., `user_avatars/profiles`)
- `is_hash`: Optional boolean to control filename hashing (default: `true`)
  - `true` or `1`: Use SHA-256 hash as filename (e.g., `abc123def456...hash.jpg`)
  - `false` or `0`: Use original filename (sanitized for safety, e.g., `my_image.jpg`)

**Example with original filename:**
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@my_document.pdf" \
  -F "bucket=my-bucket" \
  -F "path=documents" \
  -F "is_hash=false" \
  http://localhost:8080/api/v2/upload/file
```

**Response:**
```json
{
  "file_path": "documents/my_document.pdf",
  "file_hash": "abc123def456...hash",
  "message": "File uploaded successfully",
  "bucket": "my-bucket",
  "content_type": "application/pdf",
  "size": 204800,
  "duplicated": false
}
```

---

### GET /api/v2/upload/file

**Retrieve a file from storage**

**Request:**
```bash
curl -X GET \
  -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/v2/upload/file?bucket=my-bucket&file_path=user_avatars/profiles/abc123.jpg"
```

**Response:** Returns the file with appropriate content-type header

**Parameters:**
- `bucket`: Bucket name (required)
- `file_path`: Full path to the file (required)

---

### DELETE /api/v2/upload/file

**Delete a file from storage**

**Request:**
```bash
curl -X DELETE \
  -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/v2/upload/file?bucket=my-bucket&file_path=user_avatars/profiles/abc123.jpg"
```

---

### GET /api/v2/upload/files/list

**List files in a bucket with optional prefix filter**

**Request:**
```bash
curl -X GET \
  -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/v2/upload/files/list?bucket=my-bucket&prefix=user_avatars"
```

---

## Configuration | Cấu hình

### Environment Variables | Biến môi trường

| Variable | Description | Default |
|----------|-------------|---------|
| `IMAGE_MAX_SIZE` | Maximum image size in bytes | 5242880 (5MB) |
| `FILE_MAX_SIZE` | Maximum file size in bytes | 10485760 (10MB) |
| `MINIO_ENDPOINT` | MinIO/S3 endpoint URL | - |
| `MINIO_ACCESS_KEY_ID` | MinIO access key | - |
| `MINIO_SECRET_ACCESS_KEY` | MinIO secret key | - |
| `MINIO_REGION` | MinIO region | us-east-1 |
| `MINIO_USE_SSL` | Use SSL for MinIO connection | false |
| `PRIVATE_KEY` | Authentication key for private middleware | - |
| `GRAFANA_OTLP_ENDPOINT` | Grafana OTLP endpoint for logging | - |
| `SERVICE_NAME` | Service name for logging | gau-upload-service |

---

## Troubleshooting | Khắc phục sự cố

### ❌ Error 404: NoSuchKey when accessing files via CDN

**English:**

**Problem:** Files uploaded successfully to MinIO but return 404 when accessed via CDN service.

**Common Causes:**

1. **Bucket name mismatch**: Upload service saves to bucket A, but CDN reads from bucket B
   - **Solution**: Ensure both services use the same bucket name
   - Check logs: `[Upload File] File uploaded successfully: path/to/file.jpg`
   - Verify CDN is querying the same bucket

2. **Path mismatch**: File path doesn't match between upload and retrieval
   - **Upload returns**: `"file_path": "user_avatars/abc123.jpg"`
   - **CDN must use exact path**: `user_avatars/abc123.jpg` (no leading `/`)
   - **Solution**: Store and use the exact `file_path` from upload response

3. **Folder markers interference** (Fixed in latest version):
   - Previous versions created empty folder objects (e.g., `abc/`, `def/`)
   - These could confuse some CDN configurations
   - **Solution**: Update to latest version (folder markers removed)

4. **Path normalization issues**:
   - Ensure no leading/trailing slashes in file_path
   - Use forward slashes `/`, not backslashes `\`
   - Example: ✅ `folder/file.jpg` ❌ `/folder/file.jpg` ❌ `folder\file.jpg`

**Debugging Steps:**

1. Check upload service logs for the exact bucket and path:
   ```
   [Upload File] File uploaded successfully: user_avatars/abc123.jpg (hash: abc123...)
   ```

2. Check CDN service logs for the bucket and path it's requesting:
   ```
   [Get File] Request received - Bucket: my-bucket, Path: user_avatars/abc123.jpg
   ```

3. Verify file exists in MinIO using MinIO Console or CLI:
   ```bash
   mc ls myminio/my-bucket/user_avatars/
   ```

4. Test direct retrieval via upload service API:
   ```bash
   curl "http://upload-service/api/v2/upload/file?bucket=my-bucket&file_path=user_avatars/abc123.jpg"
   ```

5. Compare the paths - they must match exactly (case-sensitive)

**Tiếng Việt:**

**Vấn đề:** File upload thành công lên MinIO nhưng trả về 404 khi truy cập qua CDN service.

**Nguyên nhân phổ biến:**

1. **Bucket name không khớp**: Upload service lưu vào bucket A, CDN đọc từ bucket B
   - **Giải pháp**: Đảm bảo cả 2 service dùng cùng tên bucket
   - Kiểm tra log: `[Upload File] File uploaded successfully: path/to/file.jpg`
   - Xác minh CDN đang query đúng bucket

2. **Path không khớp**: Đường dẫn file không giống nhau giữa upload và truy xuất
   - **Upload trả về**: `"file_path": "user_avatars/abc123.jpg"`
   - **CDN phải dùng path chính xác**: `user_avatars/abc123.jpg` (không có `/` ở đầu)
   - **Giải pháp**: Lưu và dùng đúng `file_path` từ response upload

3. **Folder markers gây nhiễu** (Đã fix ở phiên bản mới):
   - Phiên bản cũ tạo các object folder rỗng (vd: `abc/`, `def/`)
   - Có thể gây nhầm lẫn cho một số cấu hình CDN
   - **Giải pháp**: Update lên phiên bản mới (đã xóa folder markers)

4. **Vấn đề chuẩn hóa path**:
   - Đảm bảo không có dấu `/` ở đầu/cuối file_path
   - Dùng dấu gạch chéo `/`, không dùng `\`
   - Ví dụ: ✅ `folder/file.jpg` ❌ `/folder/file.jpg` ❌ `folder\file.jpg`

**Các bước debug:**

1. Kiểm tra log upload service để biết chính xác bucket và path:
   ```
   [Upload File] File uploaded successfully: user_avatars/abc123.jpg (hash: abc123...)
   ```

2. Kiểm tra log CDN service để biết bucket và path nó đang request:
   ```
   [Get File] Request received - Bucket: my-bucket, Path: user_avatars/abc123.jpg
   ```

3. Xác minh file tồn tại trong MinIO bằng Console hoặc CLI:
   ```bash
   mc ls myminio/my-bucket/user_avatars/
   ```

4. Test truy xuất trực tiếp qua API upload service:
   ```bash
   curl "http://upload-service/api/v2/upload/file?bucket=my-bucket&file_path=user_avatars/abc123.jpg"
   ```

5. So sánh các path - chúng phải khớp chính xác (phân biệt chữ hoa/thường)

---

## Deployment | Triển khai

### 🐳 Docker

**English:**
1. Build the Docker image:
   ```bash
   docker build -t upload-service .
   ```
2. Run the container:
   ```bash
   docker run -p 8080:8080 \
     -e MINIO_ENDPOINT=http://minio:9000 \
     -e MINIO_ACCESS_KEY_ID=minioadmin \
     -e MINIO_SECRET_ACCESS_KEY=minioadmin \
     -e PRIVATE_KEY=your-secret-key \
     upload-service
   ```

**Tiếng Việt:**
1. Build image Docker:
   ```bash
   docker build -t upload-service .
   ```
2. Chạy container:
   ```bash
   docker run -p 8080:8080 \
     -e MINIO_ENDPOINT=http://minio:9000 \
     -e MINIO_ACCESS_KEY_ID=minioadmin \
     -e MINIO_SECRET_ACCESS_KEY=minioadmin \
     -e PRIVATE_KEY=your-secret-key \
     upload-service
   ```

---

### ☸ Kubernetes

**English:**
1. Edit environment variables in `deploy/k8s/staging/template/configmap.yaml` and `secret.yaml`.
2. Apply manifests:
   ```bash
   cd deploy/k8s/staging
   ./apply.sh
   ```
3. To remove:
   ```bash
   ./unapply.sh
   ```

**Tiếng Việt:**
1. Chỉnh sửa biến môi trường trong `deploy/k8s/staging/template/configmap.yaml` và `secret.yaml`.
2. Áp dụng manifest:
   ```bash
   cd deploy/k8s/staging
   ./apply.sh
   ```
3. Để xóa:
   ```bash
   ./unapply.sh
   ```

---

## Contact | Liên hệ

Nếu bạn có bất kỳ câu hỏi hoặc đề xuất nào, vui lòng liên hệ qua:

* Github: [tnqbao](https://github.com/tnqbao)
* LinkedIn: [https://www.linkedin.com/in/tnqb2004/](https://www.linkedin.com/in/tnqb2004/)
