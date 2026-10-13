# alert-chart-renderer

Lightweight, high-performance time-series chart snapshot renderer written in pure Go for observability alerts and interactive notification cards.

[![Go Report Card](https://goreportcard.com/badge/github.com/apack1001/alert-chart-renderer)](https://goreportcard.com/report/github.com/apack1001/alert-chart-renderer)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Features

- **Pure Go (Zero CGO / Zero External Binary)**: Renders high-fidelity PNG chart snapshots without Chrome, Headless browsers, or Python.
- **Anti-Stampede Protection**: Employs `singleflight.Group` to collapse high-concurrency alert surges into single rendering computations.
- **In-Memory LRU Cache with TTL**: Reuses recent chart outputs instantly, saving CPU and network resources.
- **Concurrency Rate Limiting**: Built-in semaphore limiter prevents OOM under alert storm conditions.
- **Interactive Visuals**:
  - Multi-series time-curve plotting with automatic scaling
  - Warning/Critical threshold dashed lines
  - Dark & Light theme presets
  - Data URL Base64 support for direct markdown embedding in Feishu / Lark cards

## Installation

```bash
go get github.com/apack1001/alert-chart-renderer
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	alertchartrenderer "github.com/apack1001/alert-chart-renderer"
)

func main() {
	svc := alertchartrenderer.NewService(alertchartrenderer.DefaultConfig())

	now := time.Now().Unix()
	opt := alertchartrenderer.RenderOptions{
		Title:    "Node CPU Usage Alert",
		Subtitle: "Instance: 192.168.1.100 (zone-a)",
		Width:    800,
		Height:   360,
		Theme:    "dark",
		Unit:     "%",
		Series: []alertchartrenderer.TimeSeries{
			{
				Name:  "cpu_utilization",
				Color: "#5794F2",
				Points: []alertchartrenderer.DataPoint{
					{Timestamp: now - 300, Value: 62.4},
					{Timestamp: now - 240, Value: 71.0},
					{Timestamp: now - 180, Value: 85.3},
					{Timestamp: now - 120, Value: 92.1},
					{Timestamp: now - 60, Value: 96.8},
					{Timestamp: now, Value: 94.5},
				},
			},
		},
		Thresholds: []alertchartrenderer.ThresholdLine{
			{
				Value: 90.0,
				Label: "Critical (90%)",
				Color: "#F2495C",
			},
		},
	}

	result, err := svc.Render(context.Background(), opt)
	if err != nil {
		panic(err)
	}

	_ = os.WriteFile("snapshot.png", result.ImageData, 0644)
	fmt.Printf("Rendered in %dms, Base64 prefix: %s\n", result.RenderDurationMs, result.Base64[:30])
}
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
