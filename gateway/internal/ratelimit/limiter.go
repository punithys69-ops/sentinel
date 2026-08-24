package ratelimit

import "sync"

// Limiter manages one token bucket per client.
type Limiter struct {
	mu         sync.Mutex
	buckets    map[string]*TokenBucket
	capacity   float64
	refillRate float64
}

// NewLimiter creates a limiter that gives every new client
// a token bucket with the same configuration.
func NewLimiter(capacity, refillRate float64) (*Limiter, error) {
	// Validate configuration by creating one temporary bucket.
	if _, err := NewTokenBucket(capacity, refillRate); err != nil {
		return nil, err
	}

	return &Limiter{
		buckets:    make(map[string]*TokenBucket),
		capacity:   capacity,
		refillRate: refillRate,
	}, nil
}

// getBucket returns the bucket belonging to a client.
// If the client has never been seen before, a new bucket is created.
func (l *Limiter) getBucket(clientKey string) (*TokenBucket, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if bucket, ok := l.buckets[clientKey]; ok {
		return bucket, nil
	}

	bucket, err := NewTokenBucket(l.capacity, l.refillRate)
	if err != nil {
		return nil, err
	}

	l.buckets[clientKey] = bucket
	return bucket, nil
}

// Allow checks whether this client may make one more request.
func (l *Limiter) Allow(clientKey string) (bool, error) {
	bucket, err := l.getBucket(clientKey)
	if err != nil {
		return false, err
	}

	return bucket.Allow(), nil
}

// RetryAfter returns an estimate of how long this client should wait.
func (l *Limiter) RetryAfter(clientKey string) (float64, error) {
	bucket, err := l.getBucket(clientKey)
	if err != nil {
		return 0, err
	}

	return bucket.RetryAfter().Seconds(), nil
}
