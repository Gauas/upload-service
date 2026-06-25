package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) writeTempFile(r io.Reader) (*os.File, func(), error) {
	dir := s.Config.TempDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("service: create temp dir: %w", err)
	}

	f, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		return nil, nil, fmt.Errorf("service: create temp file: %w", err)
	}

	cleanup := func() { f.Close(); os.Remove(f.Name()) }

	if _, err := io.Copy(f, r); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("service: write temp file: %w", err)
	}

	return f, cleanup, nil
}


func detectContentType(f *os.File) (string, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return "", fmt.Errorf("service: seek for content-type: %w", err)
	}

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("service: read for content-type: %w", err)
	}

	return http.DetectContentType(buf[:n]), nil
}

func normalizePath(p string) string {
	p = strings.Trim(p, "/\\")
	p = strings.ReplaceAll(p, "\\", "/")
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if strings.Contains(p, "..") {
		return ""
	}
	return p
}

func pathSegments(path string) []string {
	parts := strings.Split(path, "/")
	var segs []string
	for i := range parts {
		segs = append(segs, strings.Join(parts[:i+1], "/"))
	}
	return segs
}

func extensionFrom(filename, contentType string) string {
	if ext := filepath.Ext(filename); ext != "" {
		return ext
	}
	return extensionFromContentType(contentType)
}

func buildFileName(isHash bool, fileHash, original, ext string) string {
	if isHash {
		return fileHash + ext
	}
	return original
}

func buildFullPath(customPath, fileName string) string {
	if customPath != "" {
		return customPath + "/" + fileName
	}
	return fileName
}

func extensionFromContentType(ct string) string {
	switch ct {
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
