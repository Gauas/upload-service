package infra

import (
	"github.com/gauas/upload-service/config"
)

type Infra struct {
	MinioClient    *MinioClient
	ParquetService *ParquetService
	Logger         *LoggerClient
	RabbitMQ       *RabbitMQClient
}

func InitInfra(config *config.Config) *Infra {
	minioClient, err := NewMinioClient(config)
	if err != nil {
		panic("Failed to create MinIO client: " + err.Error())
	}

	parquetService := NewParquetService(minioClient)

	loggerClient := InitLoggerClient(config)
	if loggerClient == nil {
		panic("Failed to create Logger client")
	}

	rabbitMQ := InitRabbitMQClient(config)

	return &Infra{
		MinioClient:    minioClient,
		ParquetService: parquetService,
		Logger:         loggerClient,
		RabbitMQ:       rabbitMQ,
	}
}

func InitInfraForConsumer(config *config.Config) *Infra {
	minioClient, err := NewMinioClient(config)
	if err != nil {
		panic("Failed to create MinIO client: " + err.Error())
	}

	parquetService := NewParquetService(minioClient)

	loggerClient := InitLoggerClient(config)
	if loggerClient == nil {
		panic("Failed to create Logger client")
	}

	rabbitMQ := InitRabbitMQClient(config)
	if rabbitMQ == nil {
		panic("Failed to initialize RabbitMQ - required for consumer service")
	}

	return &Infra{
		MinioClient:    minioClient,
		ParquetService: parquetService,
		Logger:         loggerClient,
		RabbitMQ:       rabbitMQ,
	}
}
