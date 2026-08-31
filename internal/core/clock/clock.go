package clock

import (
	"sync"
	"time"
)

// Adjustable is a process-local clock intended for integration testing.
// A configured time keeps advancing at the same rate as the system clock.
type Adjustable struct {
	mu       sync.RWMutex
	offset   time.Duration
	adjusted bool
}

func NewAdjustable() *Adjustable { return &Adjustable{} }

func (c *Adjustable) Now() time.Time {
	c.mu.RLock()
	offset := c.offset
	c.mu.RUnlock()
	return time.Now().Add(offset)
}

func (c *Adjustable) Set(value time.Time) {
	c.mu.Lock()
	c.offset = value.Sub(time.Now())
	c.adjusted = true
	c.mu.Unlock()
}

func (c *Adjustable) Reset() {
	c.mu.Lock()
	c.offset = 0
	c.adjusted = false
	c.mu.Unlock()
}

func (c *Adjustable) Adjusted() bool {
	c.mu.RLock()
	adjusted := c.adjusted
	c.mu.RUnlock()
	return adjusted
}
