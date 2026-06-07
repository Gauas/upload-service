package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

func CheckFileType(contentType string, allowedTypes []string) bool {
	for _, t := range allowedTypes {
		if t == contentType {
			return true
		}
	}
	return false
}

func IsFileSizeAllowed(size int64, maxSizeMB int64) bool {
	return size <= maxSizeMB*1024*1024
}

func SanitizeFileName(filename string) string {
	if filename == "" {
		return "file"
	}

	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	reg := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	cleanName := reg.ReplaceAllString(nameWithoutExt, "_")

	reg2 := regexp.MustCompile(`_+`)
	cleanName = reg2.ReplaceAllString(cleanName, "_")
	cleanName = strings.Trim(cleanName, "_")

	if cleanName == "" {
		cleanName = "file"
	}

	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	if ext != "" {
		cleanExt := reg.ReplaceAllString(ext[1:], "")
		if cleanExt != "" {
			ext = "." + cleanExt
		} else {
			ext = ""
		}
	}

	return cleanName + ext
}
