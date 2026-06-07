package infra

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/gauas/upload-service/config"
)

type RabbitMQClient struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
}

func InitRabbitMQClient(cfg *config.Config) *RabbitMQClient {
	rabbitUser := cfg.Queue.Username
	rabbitPassword := cfg.Queue.Password
	rabbitHost := cfg.Queue.Host
	rabbitPort := cfg.Queue.Port

	if rabbitUser == "" || rabbitPassword == "" || rabbitHost == "" || rabbitPort == "" {
		log.Println("Warning: One or more RabbitMQ config values are missing, using defaults")
		if rabbitUser == "" {
			rabbitUser = ""
		}
		if rabbitPassword == "" {
			rabbitPassword = ""
		}
		if rabbitHost == "" {
			rabbitHost = "localhost"
		}
		if rabbitPort == "" {
			rabbitPort = "5672"
		}
	}

	dsn := fmt.Sprintf("amqp://%s:%s@%s:%s/", rabbitUser, rabbitPassword, rabbitHost, rabbitPort)

	conn, err := amqp.Dial(dsn)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ: %v", err)
		return nil
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		log.Printf("Failed to open a channel: %v", err)
		return nil
	}

	log.Println("RabbitMQ connected at", rabbitHost)
	return &RabbitMQClient{Connection: conn, Channel: ch}
}

func (r *RabbitMQClient) Close() {
	if r.Channel != nil {
		r.Channel.Close()
	}
	if r.Connection != nil {
		r.Connection.Close()
	}
	log.Println("RabbitMQ connection closed")
}

func (r *RabbitMQClient) DeclareQueue(queueName string, durable, autoDelete bool) error {
	_, err := r.Channel.QueueDeclare(queueName, durable, autoDelete, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}
	log.Printf("Queue declared: %s", queueName)
	return nil
}

func (r *RabbitMQClient) DeclareExchange(exchangeName, exchangeType string, durable bool) error {
	err := r.Channel.ExchangeDeclare(exchangeName, exchangeType, durable, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", exchangeName, err)
	}
	log.Printf("Exchange declared: %s (type: %s)", exchangeName, exchangeType)
	return nil
}

func (r *RabbitMQClient) BindQueue(queueName, exchangeName, routingKey string) error {
	err := r.Channel.QueueBind(queueName, routingKey, exchangeName, false, nil)
	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", queueName, exchangeName, err)
	}
	log.Printf("Queue %s bound to exchange %s with routing key %s", queueName, exchangeName, routingKey)
	return nil
}

func (r *RabbitMQClient) Consume(queueName, consumerTag string) (<-chan amqp.Delivery, error) {
	msgs, err := r.Channel.Consume(queueName, consumerTag, false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to register consumer for queue %s: %w", queueName, err)
	}
	log.Printf("Consumer registered for queue: %s", queueName)
	return msgs, nil
}

func (r *RabbitMQClient) PublishToExchange(exchange, routingKey string, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return r.Channel.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
	})
}
