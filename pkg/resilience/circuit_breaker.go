package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

const (
	StateClosed int32 = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	state            int32 // Atomic
	failureCount     int32
	successCount     int32
	threshold        int32
	window           time.Duration
	openTimeout      time.Duration
	successThreshold int32
	lastFailureTime  atomic.Value // time.Time
	halfOpenInFlight int32
	mu               sync.Mutex
}

func NewCircuitBreaker(threshold int32, window, openTimeout time.Duration, successThreshold int32) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:            StateClosed,
		threshold:        threshold,
		window:           window,
		openTimeout:      openTimeout,
		successThreshold: successThreshold,
	}
	cb.lastFailureTime.Store(time.Time{})
	return cb
}

func (cb *CircuitBreaker) GetState() int32 {
	return atomic.LoadInt32(&cb.state)
}

func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	state := cb.GetState()

	if state == StateOpen {
		lastFailure := cb.lastFailureTime.Load().(time.Time)
		if time.Since(lastFailure) > cb.openTimeout {
			// Transition to HalfOpen atomically
			cb.mu.Lock()
			if atomic.LoadInt32(&cb.state) == StateOpen {
				atomic.StoreInt32(&cb.state, StateHalfOpen)
				atomic.StoreInt32(&cb.successCount, 0)
			}
			cb.mu.Unlock()
			state = cb.GetState()
		} else {
			return ErrCircuitOpen
		}
	}

	if state == StateHalfOpen {
		// Strict HalfOpen Floodgate: only 1 in-flight probe
		if !atomic.CompareAndSwapInt32(&cb.halfOpenInFlight, 0, 1) {
			return ErrCircuitOpen
		}
		defer atomic.StoreInt32(&cb.halfOpenInFlight, 0)
	}

	err := fn()

	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime.Store(time.Now())

	if atomic.LoadInt32(&cb.state) == StateHalfOpen {
		atomic.StoreInt32(&cb.state, StateOpen)
		return
	}

	count := atomic.AddInt32(&cb.failureCount, 1)
	if count >= cb.threshold {
		atomic.StoreInt32(&cb.state, StateOpen)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	state := cb.GetState()
	if state == StateClosed {
		atomic.StoreInt32(&cb.failureCount, 0)
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if atomic.LoadInt32(&cb.state) == StateHalfOpen {
		count := atomic.AddInt32(&cb.successCount, 1)
		if count >= cb.successThreshold {
			cb.Reset()
		}
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	atomic.StoreInt32(&cb.state, StateClosed)
	atomic.StoreInt32(&cb.failureCount, 0)
	atomic.StoreInt32(&cb.successCount, 0)
}
