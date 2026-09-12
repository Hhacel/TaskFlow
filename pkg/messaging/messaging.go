// Package messaging provides a broker-agnostic abstraction over the
// publish/subscribe messaging used for asynchronous communication between the
// API Gateway, the Orchestrator and Workers. The NATS implementation lives in
// nats.go; consumers of this package should depend only on the Broker
// interface so the underlying transport can be swapped or mocked in tests.
package messaging

// Handler processes a single message received from the broker.
type Handler func(data []byte)

// Subscription represents an active subscription that can be cancelled.
type Subscription interface {
	Unsubscribe() error
}

// Broker is a minimal publish/subscribe abstraction used across TaskFlow
// services. Implementations must support NATS Queue Groups via
// QueueSubscribe so that multiple instances of the same consumer (e.g.
// Workers) automatically load-balance message delivery.
type Broker interface {
	// Publish sends data to the given subject.
	Publish(subject string, data []byte) error

	// Subscribe delivers every message published on subject to handler.
	Subscribe(subject string, handler Handler) (Subscription, error)

	// QueueSubscribe delivers messages published on subject to handler,
	// load-balanced across every subscriber sharing the same queueGroup.
	QueueSubscribe(subject, queueGroup string, handler Handler) (Subscription, error)

	// Close releases the underlying connection.
	Close() error
}
