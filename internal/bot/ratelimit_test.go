package bot

import (
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	r := NewRateLimiter(3)
	id := int64(123)
	if !r.Allow(id) || !r.Allow(id) || !r.Allow(id) {
		t.Fatal("3 pertama harus lolos")
	}
	if r.Allow(id) {
		t.Fatal("ke-4 harus ditolak!")
	}
	// User lain tidak terpengaruh.
	if !r.Allow(int64(456)) {
		t.Fatal("user lain harus lolos")
	}
}

func TestRateLimiterWindow(t *testing.T) {
	r := NewRateLimiter(1)
	id := int64(789)
	if !r.Allow(id) {
		t.Fatal("pertama harus lolos")
	}
	if r.Allow(id) {
		t.Fatal("kedua langsung harus ditolak")
	}
	// Mundurkan waktu manual (simulasi lewat 1 menit).
	r.mu.Lock()
	r.hits[id] = []time.Time{time.Now().Add(-2 * time.Minute)}
	r.mu.Unlock()
	if !r.Allow(id) {
		t.Fatal("setelah jendela harus lolos lagi")
	}
}
