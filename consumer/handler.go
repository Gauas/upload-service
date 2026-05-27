package main

import (
	"context"

	"github.com/gauas/upload-service/consumer/topic"
	"github.com/gauas/upload-service/infra"
)

func handleChunkComplete(ctx context.Context, i *infra.Infra, body []byte) error {
	h := topic.NewChunkCompleteHandler(i)
	return h.HandleChunkComplete(ctx, body)
}
