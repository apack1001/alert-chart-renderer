package alertchartrenderer_test

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	alertchartrenderer "github.com/apack1001/alert-chart-renderer"
	"github.com/apack1001/alert-chart-renderer/cache"
	"github.com/apack1001/alert-chart-renderer/ratelimit"
)

var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

func sampleOptions() alertchartrenderer.RenderOptions {
	now := time.Now().Unix()
	return alertchartrenderer.RenderOptions{
		Title:    "Node CPU Usage Peak",
		Subtitle: "Instance: 10.0.0.1 (cluster-prod-01)",
		Width:    800,
		Height:   360,
		Theme:    "dark",
		Unit:     "%",
		Series: []alertchartrenderer.TimeSeries{
			{
				Name:  "cpu_system",
				Color: "#5794F2",
				Points: []alertchartrenderer.DataPoint{
					{Timestamp: now - 300, Value: 45.2},
					{Timestamp: now - 240, Value: 52.8},
					{Timestamp: now - 180, Value: 68.1},
					{Timestamp: now - 120, Value: 83.5},
					{Timestamp: now - 60, Value: 91.4},
					{Timestamp: now, Value: 89.0},
				},
			},
		},
		Thresholds: []alertchartrenderer.ThresholdLine{
			{
				Value: 80.0,
				Label: "Critical",
				Color: "#F2495C",
			},
		},
	}
}

func TestService_Render(t *testing.T) {
	svc := alertchartrenderer.NewService(alertchartrenderer.DefaultConfig())
	opt := sampleOptions()

	result, err := svc.Render(context.Background(), opt)
	if err != nil {
		t.Fatalf("failed to render chart: %v", err)
	}

	if len(result.ImageData) == 0 {
		t.Fatalf("expected non-empty image data")
	}

	if !bytes.HasPrefix(result.ImageData, pngMagic) {
		t.Errorf("image data does not have valid PNG magic header")
	}

	if !strings.HasPrefix(result.Base64, "data:image/png;base64,") {
		t.Errorf("expected data URL base64 prefix")
	}

	if result.CacheHit {
		t.Errorf("expected first render to be cache miss")
	}
}

func TestService_CacheAndSingleflight(t *testing.T) {
	svc := alertchartrenderer.NewService(alertchartrenderer.Config{
		CacheCapacity: 100,
		CacheTTL:      10 * time.Second,
		MaxConcurrent: 5,
	})
	opt := sampleOptions()

	// Initial render
	res1, err := svc.Render(context.Background(), opt)
	if err != nil {
		t.Fatalf("render 1 failed: %v", err)
	}
	if res1.CacheHit {
		t.Errorf("expected first call to not hit cache")
	}

	// Immediate subsequent render should hit cache
	res2, err := svc.Render(context.Background(), opt)
	if err != nil {
		t.Fatalf("render 2 failed: %v", err)
	}
	if !res2.CacheHit {
		t.Errorf("expected second call to hit cache")
	}

	// Concurrent bursts of identical request
	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			r, err := svc.Render(context.Background(), opt)
			if err != nil {
				t.Errorf("concurrent render failed: %v", err)
			}
			if len(r.ImageData) == 0 {
				t.Errorf("expected valid image data")
			}
		}()
	}
	wg.Wait()
}

func TestRenderer_ThemesAndEmptyData(t *testing.T) {
	optDark := sampleOptions()
	optDark.Theme = "dark"
	imgDark, err := alertchartrenderer.RenderChart(optDark)
	if err != nil {
		t.Fatalf("dark theme render failed: %v", err)
	}
	if !bytes.HasPrefix(imgDark, pngMagic) {
		t.Errorf("dark theme invalid PNG")
	}

	optLight := sampleOptions()
	optLight.Theme = "light"
	imgLight, err := alertchartrenderer.RenderChart(optLight)
	if err != nil {
		t.Fatalf("light theme render failed: %v", err)
	}
	if !bytes.HasPrefix(imgLight, pngMagic) {
		t.Errorf("light theme invalid PNG")
	}

	// Empty data rendering
	optEmpty := alertchartrenderer.RenderOptions{
		Title: "Empty Chart",
	}
	imgEmpty, err := alertchartrenderer.RenderChart(optEmpty)
	if err != nil {
		t.Fatalf("empty chart render failed: %v", err)
	}
	if !bytes.HasPrefix(imgEmpty, pngMagic) {
		t.Errorf("empty chart invalid PNG")
	}
}

func TestLRUCache(t *testing.T) {
	c := cache.NewLRUCache(2)
	c.Set("k1", []byte("v1"), time.Minute)
	c.Set("k2", []byte("v2"), time.Minute)

	v, ok := c.Get("k1")
	if !ok || string(v) != "v1" {
		t.Fatalf("expected k1 to be v1")
	}

	// Add k3, k2 should be evicted because k1 was accessed
	c.Set("k3", []byte("v3"), time.Minute)
	if _, ok := c.Get("k2"); ok {
		t.Errorf("expected k2 to be evicted")
	}

	// Test expiration
	c.Set("kExp", []byte("vExp"), 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, ok := c.Get("kExp"); ok {
		t.Errorf("expected kExp to be expired")
	}
}

func TestLimiter(t *testing.T) {
	limiter := ratelimit.NewLimiter(1)

	if !limiter.TryAcquire() {
		t.Fatalf("expected TryAcquire to succeed")
	}
	if limiter.TryAcquire() {
		t.Fatalf("expected second TryAcquire to fail")
	}

	limiter.Release()
	if err := limiter.Acquire(context.Background(), time.Second); err != nil {
		t.Fatalf("expected acquire after release to succeed: %v", err)
	}
	limiter.Release()
}
