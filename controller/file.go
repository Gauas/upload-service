package controller

import (
	"strings"

	"github.com/gauas/upload-service/packages/httpresp"
	"github.com/gauas/upload-service/service"
	"github.com/labstack/echo/v4"
)

func (ctrl *Controller) UploadFile(c echo.Context) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return httpresp.NewError(400, "file is required")
	}

	bucket := strings.TrimSpace(c.FormValue("bucket"))
	if bucket == "" {
		return httpresp.NewError(400, "bucket is required")
	}

	path := strings.TrimSpace(c.FormValue("path"))
	isHashStr := strings.ToLower(strings.TrimSpace(c.FormValue("is_hash")))
	isHash := isHashStr != "false" && isHashStr != "0"

	src, err := fileHeader.Open()
	if err != nil {
		return httpresp.NewError(500, "failed to open file")
	}
	defer src.Close()

	result, err := ctrl.service.Upload(c.Request().Context(), service.UploadParams{
		Reader:      src,
		Size:        fileHeader.Size,
		Filename:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Bucket:      bucket,
		Path:        path,
		IsHash:      isHash,
	})
	if err != nil {
		return err
	}

	return httpresp.OK(c, result)
}

func (ctrl *Controller) GetFile(c echo.Context) error {
	bucket := c.QueryParam("bucket")
	path := c.QueryParam("file_path")

	if bucket == "" {
		return httpresp.NewError(400, "bucket is required")
	}
	if path == "" {
		return httpresp.NewError(400, "file_path is required")
	}

	data, contentType, err := ctrl.service.Get(c.Request().Context(), bucket, path)
	if err != nil {
		return httpresp.ErrorNotFound
	}

	return c.Blob(200, contentType, data)
}

func (ctrl *Controller) DeleteFile(c echo.Context) error {
	bucket := c.QueryParam("bucket")
	path := c.QueryParam("file_path")

	if bucket == "" {
		return httpresp.NewError(400, "bucket is required")
	}
	if path == "" {
		return httpresp.NewError(400, "file_path is required")
	}

	if err := ctrl.service.Delete(c.Request().Context(), bucket, path); err != nil {
		return err
	}

	return httpresp.NoContent(c, "file deleted")
}

func (ctrl *Controller) ListFiles(c echo.Context) error {
	bucket := c.QueryParam("bucket")
	prefix := c.QueryParam("prefix")

	if bucket == "" {
		return httpresp.NewError(400, "bucket is required")
	}

	files, err := ctrl.service.List(c.Request().Context(), bucket, prefix)
	if err != nil {
		return err
	}

	return httpresp.OK(c, echo.Map{
		"files":  files,
		"count":  len(files),
		"bucket": bucket,
		"prefix": prefix,
	})
}

func (ctrl *Controller) Health(c echo.Context) error {
	return httpresp.OK(c, echo.Map{"status": "ok"})
}
