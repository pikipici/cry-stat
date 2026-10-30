// Package bot: rate limiter per akun (bab 9 plan — wajib Stage 1).
// Token bucket sederhana: N perintah per 60 detik rolling per telegram ID.
package bot

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu     sync.Mutex
	hits   map[int64][]time.Time
	perMin int
}

func NewRateLimiter(perMin int) *RateLimiter {
	if perMin <= 0 {
		perMin = 30
	}
	return &RateLimiter{hits: map[int64][]time.Time{}, perMin: perMin}
}

func (r *RateLimiter) SetLimit(n int) {
	r.mu.Lock()
	r.perMin = n
	r.mu.Unlock()
}

// Allow: true = boleh jalan. False = rate limited (catat RiskEvent di caller).
func (r *RateLimiter) Allow(id int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cut := now.Add(-time.Minute)
	hs := r.hits[id]
	keep := hs[:0]
	for _, t := range hs {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	if len(keep) >= r.perMin {
		r.hits[id] = keep
		return false
	}
	r.hits[id] = append(keep, now)
	return true
}
