package messaging

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSConfig holds connection settings for the NATS broker implementation.
type NATSConfig struct {
	URL           string `yaml:"url"`
	ReconnectWait int    `yaml:"reconnect_wait_seconds"`
	MaxReconnects int    `yaml:"max_reconnects"`
}

// natsBroker is a Broker implementation backed by a real NATS connection.
type natsBroker struct {
	conn *nats.Conn
}

// NewNATSBroker connects to NATS and returns a Broker backed by that connection.
func NewNATSBroker(cfg NATSConfig) (Broker, error) {
	opts := []nats.Option{
		nats.ReconnectWait(time.Duration(cfg.ReconnectWait) * time.Second),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				slog.Warn("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	slog.Info("Connected to NATS", "url", cfg.URL)

	return &natsBroker{conn: conn}, nil
}

func (b *natsBroker) Publish(subject string, data []byte) error {
	return b.conn.Publish(subject, data)
}

func (b *natsBroker) Subscribe(subject string, handler Handler) (Subscription, error) {
	sub, err := b.conn.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (b *natsBroker) QueueSubscribe(subject, queueGroup string, handler Handler) (Subscription, error) {
	sub, err := b.conn.QueueSubscribe(subject, queueGroup, func(msg *nats.Msg) {
		handler(msg.Data)
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (b *natsBroker) Close() error {
	if b.conn != nil {
		b.conn.Close()
	}
	return nil
}
