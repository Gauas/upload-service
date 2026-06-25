package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/infra"
)

const (
	exchange              = "upload.exchange"
	chunkCompleteQueue    = "upload.chunk_complete"
	composeCompletedQueue = "upload.compose_completed"
	consumerTag           = "gau-upload-consumer"
)

func main() {
	cfg := config.New()

	infraInstance := infra.NewForConsumer(cfg)
	defer infraInstance.Queue.Close()

	setupQueues(infraInstance)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgs, err := infraInstance.Queue.Consume(chunkCompleteQueue, consumerTag)
	if err != nil {
		log.Fatalf("consumer: consume: %v", err)
	}

	go func() {
		log.Printf("consumer: listening on %s", chunkCompleteQueue)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				if err := handleChunkComplete(ctx, infraInstance, msg.Body); err != nil {
					log.Printf("consumer: handle error: %v", err)
					_ = msg.Nack(false, false)
					continue
				}
				_ = msg.Ack(false)
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("consumer: shutting down")
}

func setupQueues(i *infra.Infra) {
	if err := i.Queue.DeclareExchange(exchange, "topic", true); err != nil {
		log.Fatalf("consumer: declare exchange: %v", err)
	}

	queues := []struct{ name, key string }{
		{chunkCompleteQueue, "upload.chunk_complete"},
		{composeCompletedQueue, "upload.compose_completed"},
	}

	for _, q := range queues {
		if err := i.Queue.DeclareQueue(q.name, true, false); err != nil {
			log.Fatalf("consumer: declare queue %s: %v", q.name, err)
		}
		if err := i.Queue.Bind(q.name, exchange, q.key); err != nil {
			log.Fatalf("consumer: bind %s: %v", q.name, err)
		}
	}
}
