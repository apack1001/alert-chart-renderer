package alertchartrenderer

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/apack1001/alert-chart-renderer/cache"
	"github.com/apack1001/alert-chart-renderer/ratelimit"
)

// Service coordinates caching, singleflight deduplication, and snapshot chart rendering.
type Service struct {
	cache      *cache.LRUCache
	limiter    *ratelimit.Limiter
	sf         singleflight.Group
	defaultTTL time.Duration
}

// Config holds configuration parameters for Service.
type Config struct {
	CacheCapacity int
	CacheTTL      time.Duration
	MaxConcurrent int
}

// DefaultConfig provides recommended production defaults.
func DefaultConfig() Config {
	return Config{
		CacheCapacity: 1000,
		CacheTTL:      5 * time.Minute,
		MaxConcurrent: 16,
	}
}

// NewService instantiates a thread-safe snapshot rendering service.
func NewService(cfg Config) *Service {
	if cfg.CacheCapacity <= 0 {
		cfg.CacheCapacity = 1000
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 16
	}

	return &Service{
		cache:      cache.NewLRUCache(cfg.CacheCapacity),
		limiter:    ratelimit.NewLimiter(cfg.MaxConcurrent),
		defaultTTL: cfg.CacheTTL,
	}
}

// Render generates a chart snapshot with LRU cache and singleflight protection against stampedes.
func (s *Service) Render(ctx context.Context, opt RenderOptions) (RenderResult, error) {
	started := time.Now()
	fp := opt.Fingerprint()

	// 1. Check in-memory cache
	if cachedData, found := s.cache.Get(fp); found {
		return RenderResult{
			ImageData:        cachedData,
			MimeType:         "image/png",
			Base64:           "data:image/png;base64," + base64.StdEncoding.EncodeToString(cachedData),
			CacheHit:         true,
			RenderDurationMs: time.Since(started).Milliseconds(),
			GeneratedAt:      time.Now(),
		}, nil
	}

	// 2. Singleflight deduplication for identical concurrent requests
	v, err, _ := s.sf.Do(fp, func() (interface{}, error) {
		// Double-check cache in case previous request just completed
		if cachedData, found := s.cache.Get(fp); found {
			return cachedData, nil
		}

		// Concurrency limit acquisition
		if err := s.limiter.Acquire(ctx, 3*time.Second); err != nil {
			return nil, fmt.Errorf("failed to acquire render slot: %w", err)
		}
		defer s.limiter.Release()

		// Execute pure-Go image rendering
		imgBytes, err := RenderChart(opt)
		if err != nil {
			return nil, err
		}

		// Save to cache
		s.cache.Set(fp, imgBytes, s.defaultTTL)
		return imgBytes, nil
	})

	if err != nil {
		return RenderResult{}, err
	}

	imgBytes := v.([]byte)
	return RenderResult{
		ImageData:        imgBytes,
		MimeType:         "image/png",
		Base64:           "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes),
		CacheHit:         false,
		RenderDurationMs: time.Since(started).Milliseconds(),
		GeneratedAt:      time.Now(),
	}, nil
}
