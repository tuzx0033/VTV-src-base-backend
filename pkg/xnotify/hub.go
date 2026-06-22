package xnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"vtv.vn/backend/pkg/xlogger"
)

// CancelFunc unsubscribes a client and releases resources.
type CancelFunc func()

// Hub is the notification fan-out interface.
type Hub interface {
	// Publish sends an event to all subscribers of the given user.
	Publish(ctx context.Context, userID int64, event Event) error
	// Subscribe returns a channel that receives events for the given user.
	// The caller MUST invoke the returned CancelFunc when done.
	Subscribe(ctx context.Context, userID int64) (<-chan Event, CancelFunc, error)
	// Close shuts down the hub and all active subscriptions.
	Close() error
}

const (
	channelPrefix   = "app:notif:"
	maxConnsPerUser = 5
	chanBufSize     = 16
)

func channelName(userID int64) string {
	return fmt.Sprintf("%s%d", channelPrefix, userID)
}

// conn tracks a single subscriber channel.
type conn struct {
	ch     chan Event
	cancel context.CancelFunc
}

// RedisHub implements Hub using Redis Pub/Sub.
type RedisHub struct {
	rdb    *redis.Client
	logger *xlogger.Logger

	mu    sync.Mutex
	conns map[int64]map[*conn]struct{} // userID -> set of connections
	done  chan struct{}
}

// NewRedisHub creates a new Hub backed by the given Redis client.
func NewRedisHub(rdb *redis.Client, logger *xlogger.Logger) *RedisHub {
	return &RedisHub{
		rdb:    rdb,
		logger: logger,
		conns:  make(map[int64]map[*conn]struct{}),
		done:   make(chan struct{}),
	}
}

// Publish sends an event to all subscribers of userID via Redis PUBLISH.
// Best-effort: errors are returned but callers typically log-and-ignore.
func (h *RedisHub) Publish(ctx context.Context, userID int64, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("xnotify: marshal event: %w", err)
	}
	if err := h.rdb.Publish(ctx, channelName(userID), data).Err(); err != nil {
		return fmt.Errorf("xnotify: redis publish: %w", err)
	}
	return nil
}

// Subscribe creates a new subscription for userID. Returns a read-only channel
// and a CancelFunc that MUST be called to clean up. Returns error if the user
// already has maxConnsPerUser active connections.
func (h *RedisHub) Subscribe(ctx context.Context, userID int64) (<-chan Event, CancelFunc, error) {
	h.mu.Lock()

	// Check max connections.
	if s, ok := h.conns[userID]; ok && len(s) >= maxConnsPerUser {
		h.mu.Unlock()
		return nil, nil, fmt.Errorf("xnotify: user %d exceeds max %d connections", userID, maxConnsPerUser)
	}

	// Create subscriber.
	subCtx, subCancel := context.WithCancel(ctx)
	c := &conn{
		ch:     make(chan Event, chanBufSize),
		cancel: subCancel,
	}

	if h.conns[userID] == nil {
		h.conns[userID] = make(map[*conn]struct{})
	}
	h.conns[userID][c] = struct{}{}
	h.mu.Unlock()

	// Start Redis subscription in background goroutine.
	redisSub := h.rdb.Subscribe(subCtx, channelName(userID))

	go func() {
		defer func() {
			_ = redisSub.Close()
			close(c.ch)

			h.mu.Lock()
			if s, ok := h.conns[userID]; ok {
				delete(s, c)
				if len(s) == 0 {
					delete(h.conns, userID)
				}
			}
			h.mu.Unlock()
		}()

		ch := redisSub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return // Redis subscription closed
				}
				var ev Event
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					h.logger.Warn("xnotify: unmarshal event from redis",
						xlogger.Err(err),
						xlogger.String("payload", msg.Payload),
					)
					continue
				}
				// Non-blocking send: drop event if consumer is slow.
				select {
				case c.ch <- ev:
				default:
					h.logger.Warn("xnotify: dropping event for slow consumer",
						xlogger.Int64("userID", userID),
						xlogger.String("type", ev.Type),
					)
				}
			case <-subCtx.Done():
				return
			case <-h.done:
				return
			}
		}
	}()

	cancelFn := func() {
		subCancel()
	}

	return c.ch, cancelFn, nil
}

// Close shuts down the hub and cancels all active subscriptions.
func (h *RedisHub) Close() error {
	close(h.done)

	h.mu.Lock()
	defer h.mu.Unlock()

	for userID, conns := range h.conns {
		for c := range conns {
			c.cancel()
		}
		delete(h.conns, userID)
	}
	return nil
}
