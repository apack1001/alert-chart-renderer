package alertchartrenderer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// DataPoint represents a timestamp-value pair in a metric series.
type DataPoint struct {
	Timestamp int64   `json:"timestamp"` // Unix timestamp in seconds
	Value     float64 `json:"value"`
}

// TimeSeries represents an individual named metric curve.
type TimeSeries struct {
	Name   string      `json:"name"`
	Color  string      `json:"color,omitempty"` // Hex color code like #3274D9
	Points []DataPoint `json:"points"`
}

// ThresholdLine defines a reference or breach threshold on the chart.
type ThresholdLine struct {
	Value float64 `json:"value"`
	Label string  `json:"label"`
	Color string  `json:"color,omitempty"` // Hex color code like #E02020
}

// RenderOptions specifies configuration for snapshot generation.
type RenderOptions struct {
	Title      string          `json:"title"`
	Subtitle   string          `json:"subtitle,omitempty"`
	Width      int             `json:"width"`  // Defaults to 800
	Height     int             `json:"height"` // Defaults to 360
	Theme      string          `json:"theme"`  // "dark" or "light"
	Unit       string          `json:"unit,omitempty"`
	Series     []TimeSeries    `json:"series"`
	Thresholds []ThresholdLine `json:"thresholds,omitempty"`
}

// Fingerprint calculates a deterministic SHA256 key for caching and singleflight deduplication.
func (opt RenderOptions) Fingerprint() string {
	raw, _ := json.Marshal(opt)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

// RenderResult holds rendered image payload and diagnostics metadata.
type RenderResult struct {
	ImageData        []byte    `json:"-"`
	MimeType         string    `json:"mimeType"` // image/png
	Base64           string    `json:"base64,omitempty"`
	CacheHit         bool      `json:"cacheHit"`
	RenderDurationMs int64     `json:"renderDurationMs"`
	GeneratedAt      time.Time `json:"generatedAt"`
}
