// api/util/event_bus.go

package util

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	logger "github.com/dev-mohitbeniwal/echo/api/logging"
)

// Event represents an event in the system
type Event struct {
	Type    string
	Payload interface{}
}

// EventHandler is a function that handles an event
type EventHandler func(context.Context, Event) error

// EventBus manages event subscriptions and publications
type EventBus struct {
	subscribers map[string][]EventHandler
	mu          sync.RWMutex
	errorChan   chan error
}

// NewEventBus creates a new EventBus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]EventHandler),
		errorChan:   make(chan error, 100), // Buffer size can be adjusted
	}
}

// Subscribe adds a new subscriber for a specific event type
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
}

// Publish sends an event to all subscribers
func (eb *EventBus) Publish(ctx context.Context, eventType string, payload interface{}) {
	eb.mu.RLock()
	handlers, exists := eb.subscribers[eventType]
	eb.mu.RUnlock()

	if !exists {
		return
	}

	event := Event{
		Type:    eventType,
		Payload: payload,
	}

	for _, handler := range handlers {
		go func(h EventHandler) {
			if err := h(ctx, event); err != nil {
				select {
				case eb.errorChan <- fmt.Errorf("event handler error: %w", err):
				default:
					// If error channel is full, log the error
					logger.Error("Error channel full, logging event handler error",
						zap.Error(err),
						zap.String("eventType", eventType))
				}
			}
		}(handler)
	}
}

// Start begins processing events and handling errors
func (eb *EventBus) Start(ctx context.Context) {
	go eb.processErrors(ctx)
}

// processErrors handles errors from event handlers
func (eb *EventBus) processErrors(ctx context.Context) {
	for {
		select {
		case err := <-eb.errorChan:
			logger.Error("Event handler error", zap.Error(err))
		case <-ctx.Done():
			return
		}
	}
}

// Unsubscribe removes a subscriber for a specific event type
func (eb *EventBus) Unsubscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if handlers, exists := eb.subscribers[eventType]; exists {
		for i, h := range handlers {
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
				eb.subscribers[eventType] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}
}
