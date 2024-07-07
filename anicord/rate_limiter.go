package anicord

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

type RateLimiter struct {
	Limit     int
	Remaining int
	Reset     time.Time
	mu        sync.Mutex
}

func (r *RateLimiter) Lock() {
	r.mu.Lock()
	now := time.Now()
	if now.After(r.Reset) {
		r.Reset = now
		r.Remaining = r.Limit
	}

	if r.Remaining == 0 {
		time.Sleep(r.Reset.Sub(now))
	}
	r.Remaining--
}

func (r *RateLimiter) Unlock(rs *http.Response) error {
	defer r.mu.Unlock()
	if rs == nil {
		return nil
	}

	var (
		limit      = rs.Header.Get("X-RateLimit-Limit")
		remaining  = rs.Header.Get("X-RateLimit-Remaining")
		retryAfter = rs.Header.Get("Retry-After")
	)

	var err error
	r.Limit, err = strconv.Atoi(limit)
	if err != nil {
		return err
	}
	r.Remaining, err = strconv.Atoi(remaining)
	if err != nil {
		return err
	}
	if retryAfter != "" {
		var after int
		after, err = strconv.Atoi(retryAfter)
		if err != nil {
			return err
		}
		r.Reset = time.Now().Add(time.Duration(after) * time.Second)
	}

	return nil
}
