package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/gauas/upload-service/config"
)

type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func New(cfg config.QueueConfig) (*Client, error) {
	dsn := fmt.Sprintf("amqp://%s:%s@%s:%s/", cfg.Username, cfg.Password, cfg.Host, cfg.Port)

	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, fmt.Errorf("queue: dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("queue: open channel: %w", err)
	}

	log.Printf("queue: connected at %s", cfg.Host)
	return &Client{conn: conn, channel: ch}, nil
}

func (c *Client) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) DeclareQueue(name string, durable, autoDelete bool) error {
	_, err := c.channel.QueueDeclare(name, durable, autoDelete, false, false, nil)
	if err != nil {
		return fmt.Errorf("queue: declare %s: %w", name, err)
	}
	return nil
}

func (c *Client) DeclareExchange(name, kind string, durable bool) error {
	err := c.channel.ExchangeDeclare(name, kind, durable, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("queue: declare exchange %s: %w", name, err)
	}
	return nil
}

func (c *Client) Bind(queue, exchange, routingKey string) error {
	err := c.channel.QueueBind(queue, routingKey, exchange, false, nil)
	if err != nil {
		return fmt.Errorf("queue: bind %s → %s: %w", queue, exchange, err)
	}
	return nil
}

func (c *Client) Consume(queue, tag string) (<-chan amqp.Delivery, error) {
	msgs, err := c.channel.Consume(queue, tag, false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("queue: consume %s: %w", queue, err)
	}
	return msgs, nil
}

func (c *Client) Publish(exchange, routingKey string, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.channel.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
	})
}
