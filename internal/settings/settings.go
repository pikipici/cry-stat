// Package settings: baca/tulis tabel settings dengan cache in-memory + reload.
// Semua angka kunci plan (spread, limit, threshold, staleness, ...) lewat sini — tanpa restart.
package settings

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/pikipici/crypto-station/internal/db"

	"github.com/shopspring/decimal"
)

type Service struct {
	q     *db.Queries
	mu    sync.RWMutex
	cache map[string]string
}

func New(q *db.Queries) *Service { return &Service{q: q, cache: map[string]string{}} }

func (s *Service) Reload(ctx context.Context) error {
	rows, err := s.q.ListSettings(ctx)
	if err != nil {
		return err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Value
	}
	s.mu.Lock()
	s.cache = m
	s.mu.Unlock()
	return nil
}

func (s *Service) Get(key string) (string, bool) {
	s.mu.RLock()
	v, ok := s.cache[key]
	s.mu.RUnlock()
	return v, ok
}

func (s *Service) MustString(key string) string {
	v, ok := s.Get(key)
	if !ok {
		panic(fmt.Sprintf("settings: key %q belum di-seed", key))
	}
	return v
}

func (s *Service) Int64(key string) int64 {
	v, err := strconv.ParseInt(s.MustString(key), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("settings: key %q bukan int: %v", key, err))
	}
	return v
}

func (s *Service) Decimal(key string) decimal.Decimal {
	d, err := decimal.NewFromString(s.MustString(key))
	if err != nil {
		panic(fmt.Sprintf("settings: key %q bukan decimal: %v", key, err))
	}
	return d
}

// Spread per coin+side dari settings (spread_buy_bnb, dst).
func (s *Service) Spread(asset, side string) decimal.Decimal {
	return s.Decimal(fmt.Sprintf("spread_%s_%s", side, asset))
}
