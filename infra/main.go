package infra

import (
	"log"

	"github.com/gauas/upload-service/config"
	"github.com/gauas/upload-service/packages/metadata"
	"github.com/gauas/upload-service/packages/queue"
	"github.com/gauas/upload-service/packages/storage"
)

type Infra struct {
	Storage  *storage.Client
	Metadata *metadata.Service
	Queue    *queue.Client
}

func New(cfg config.Config) *Infra {
	store, err := storage.New(cfg.Storage)
	if err != nil {
		log.Fatalf("infra: storage: %v", err)
	}
	log.Println("infra: storage connected")

	meta := metadata.New(store)

	q, err := queue.New(cfg.Queue)
	if err != nil {
		log.Printf("infra: queue unavailable: %v", err)
	} else {
		log.Println("infra: queue connected")
	}

	return &Infra{
		Storage:  store,
		Metadata: meta,
		Queue:    q,
	}
}

func NewForConsumer(cfg config.Config) *Infra {
	store, err := storage.New(cfg.Storage)
	if err != nil {
		log.Fatalf("infra: storage: %v", err)
	}

	meta := metadata.New(store)

	q, err := queue.New(cfg.Queue)
	if err != nil {
		log.Fatalf("infra: queue required for consumer: %v", err)
	}
	log.Println("infra: consumer ready")

	return &Infra{
		Storage:  store,
		Metadata: meta,
		Queue:    q,
	}
}
