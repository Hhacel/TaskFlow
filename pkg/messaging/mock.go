package messaging

import "sync"

// MockBroker is an in-memory Broker implementation intended for unit tests.
// Publish delivers synchronously to every matching subscriber.
type MockBroker struct {
	mu       sync.Mutex
	handlers map[string][]Handler
	Sent     []struct {
		Subject string
		Data    []byte
	}
}

// NewMockBroker creates an empty MockBroker.
func NewMockBroker() *MockBroker {
	return &MockBroker{handlers: make(map[string][]Handler)}
}

func (m *MockBroker) Publish(subject string, data []byte) error {
	m.mu.Lock()
	m.Sent = append(m.Sent, struct {
		Subject string
		Data    []byte
	}{Subject: subject, Data: data})
	handlers := append([]Handler(nil), m.handlers[subject]...)
	m.mu.Unlock()

	for _, h := range handlers {
		h(data)
	}
	return nil
}

func (m *MockBroker) Subscribe(subject string, handler Handler) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[subject] = append(m.handlers[subject], handler)
	return &mockSubscription{}, nil
}

func (m *MockBroker) QueueSubscribe(subject, _ string, handler Handler) (Subscription, error) {
	return m.Subscribe(subject, handler)
}

func (m *MockBroker) Close() error {
	return nil
}

type mockSubscription struct{}

func (mockSubscription) Unsubscribe() error { return nil }
