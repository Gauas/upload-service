package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/gauas/upload-service/consumer/topic"
	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/infra"
)

const (
	// ChunkCompleteQueue receives messages from cloud-orchestrator when all chunks are uploaded
	ChunkCompleteQueue = "upload.chunk_complete"
	ConsumerTag        = "gau-upload-consumer"

	// Exchange and routing keys
	UploadExchange             = "upload.exchange"
	ChunkCompleteRoutingKey    = "upload.chunk_complete"
	ComposeCompletedRoutingKey = "upload.compose_completed"
)

func main() {
	// Load environment variables
	if err := godotenv.Load("/gau_upload/upload.env"); err != nil {
		log.Println("No .env file found, continuing with environment variables")
	}

	// Initialize configuration
	cfgValue := config.New()
	cfg := &cfgValue
	log.Printf("Consumer service starting with config: %+v", cfg.Env)

	// Initialize infrastructure for consumer (requires RabbitMQ)
	inf := infra.InitInfraForConsumer(cfg)
	defer func() {
		if inf.RabbitMQ != nil {
			inf.RabbitMQ.Close()
		}
	}()

	// Declare exchange
	if err := inf.RabbitMQ.DeclareExchange(UploadExchange, "topic", true); err != nil {
		log.Fatalf("Failed to declare exchange: %v", err)
	}

	// Declare and bind chunk_complete queue
	if err := inf.RabbitMQ.DeclareQueue(ChunkCompleteQueue, true, false); err != nil {
		log.Fatalf("Failed to declare chunk_complete queue: %v", err)
	}
	if err := inf.RabbitMQ.BindQueue(ChunkCompleteQueue, UploadExchange, ChunkCompleteRoutingKey); err != nil {
		log.Fatalf("Failed to bind chunk_complete queue: %v", err)
	}

	// Declare compose_completed queue (for publishing back to cloud-orchestrator)
	if err := inf.RabbitMQ.DeclareQueue("upload.compose_completed", true, false); err != nil {
		log.Fatalf("Failed to declare compose_completed queue: %v", err)
	}
	if err := inf.RabbitMQ.BindQueue("upload.compose_completed", UploadExchange, ComposeCompletedRoutingKey); err != nil {
		log.Fatalf("Failed to bind compose_completed queue: %v", err)
	}

	// Create chunk complete handler
	handler := topic.NewChunkCompleteHandler(inf)

	// Start consuming chunk_complete messages
	msgs, err := inf.RabbitMQ.Consume(ChunkCompleteQueue, ConsumerTag)
	if err != nil {
		log.Fatalf("Failed to start consuming: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start message processing in goroutine
	go func() {
		log.Printf("Consumer started. Listening for chunk_complete messages on queue: %s", ChunkCompleteQueue)
		for {
			select {
			case <-ctx.Done():
				log.Println("Context cancelled, stopping consumer...")
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Message channel closed")
					return
				}

				log.Printf("Received chunk_complete message: %s", string(msg.Body))

				// Process the message
				if err := handler.HandleChunkComplete(ctx, msg.Body); err != nil {
					log.Printf("Error processing chunk_complete message: %v", err)
					// Nack the message WITHOUT requeue to avoid infinite loop
					// Failed messages should go to dead-letter queue or be logged for manual investigation
					if nackErr := msg.Nack(false, false); nackErr != nil {
						log.Printf("Failed to nack message: %v", nackErr)
					}
					continue
				}

				// Ack the message on success
				if err := msg.Ack(false); err != nil {
					log.Printf("Failed to ack message: %v", err)
				}
				log.Println("Chunk complete message processed successfully")
			}
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received")
	cancel()
	log.Println("Consumer service stopped gracefully")
}
