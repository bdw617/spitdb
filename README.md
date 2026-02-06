# SpitDB

**S**ingle **P**oint **I**n **T**ime **D**ata**B**ase - A lightweight, in-memory not-time-series time-series storage with full PromQL support.

## What is SpitDB?

SpitDB is an ephemeral metrics database that stores only the **latest value** for each unique metric series. Unlike traditional time-series databases that retain historical data, SpitDB focuses on point-in-time snapshots, making it ideal for on-demand metric aggregation and real-time queries.

## Why SpitDB?

Traditional time-series databases (Prometheus, InfluxDB, TimescaleDB) are designed for:
- Long-term metric storage
- Historical trend analysis
- Alerting based on time windows

But sometimes you need something simpler:

| Use Case | Traditional TSDB | SpitDB |
|----------|-----------------|--------|
| Store months of metrics | ✅ | ❌ |
| Query historical trends | ✅ | ❌ |
| On-demand metric aggregation | More than you need | ✅ |
| Lightweight metric proxy | Heavy setup | ✅ |
| Point-in-time snapshots | Stores more than needed | ✅ |
| Ephemeral aggregation pipelines | Heavy setup | ✅ |

### Perfect For

- **Metric Aggregators**: Collect metrics from multiple sources, aggregate with PromQL, expose combined results
- **Serverless Functions**: Query metrics without maintaining persistent storage
- **Testing**: Mock Prometheus storage for unit tests
- **Edge Computing**: Lightweight metric processing where resources are limited
- **Metric Proxies**: Transform or filter metrics on-the-fly

## Installation

```bash
go get github.com/bdw617/spitdb
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    "github.com/bdw617/spitdb"
    "github.com/prometheus/prometheus/model/labels"
)

func main() {
    // Create storage
    storage := spitdb.New()
    ctx := context.Background()

    // Write some metrics
    samples := []spitdb.Sample{
        {
            Labels: labels.FromStrings("__name__", "http_requests_total", "method", "GET", "status", "200"),
            Value:  1500,
        },
        {
            Labels: labels.FromStrings("__name__", "http_requests_total", "method", "POST", "status", "200"),
            Value:  300,
        },
        {
            Labels: labels.FromStrings("__name__", "http_requests_total", "method", "GET", "status", "500"),
            Value:  25,
        },
    }
    storage.WriteSamples(ctx, samples)

    // Query with full PromQL support
    results, _ := storage.QueryPromQL(ctx, `sum(http_requests_total) by (method)`)

    for _, r := range results {
        fmt.Printf("%v: %.0f\n", r.Labels, r.Value)
    }
    // Output:
    // map[method:GET]: 1525
    // map[method:POST]: 300
}
```

## Package Structure

SpitDB is organized into focused sub-packages:

```
github.com/bdw617/spitdb
├── pkg/db      # Core storage implementation
└── pkg/query   # PromQL query engine
```

### Root Package (Simple API)

For most use cases, import the root package:

```go
import "github.com/bdw617/spitdb"

storage := spitdb.New()
storage.WriteSamples(ctx, samples)
results, err := storage.QueryPromQL(ctx, `sum(metric) by (label)`)
```

### Sub-packages (Advanced Usage)

For more control, use the sub-packages directly:

```go
import (
    "github.com/bdw617/spitdb/pkg/db"
    "github.com/bdw617/spitdb/pkg/query"
)

// Create storage and engine separately
storage := db.New()
engine := query.NewEngine(&query.EngineOpts{
    MaxSamples: 10000000,
    Timeout:    30 * time.Second,
})

// Write to storage
storage.WriteSamples(ctx, samples)

// Query with custom engine
results, err := engine.Query(ctx, storage, `avg(metric)`)
```

## PromQL Support

SpitDB supports the full PromQL query language:

```go
// Selectors
storage.QueryPromQL(ctx, `http_requests_total`)
storage.QueryPromQL(ctx, `http_requests_total{status="500"}`)
storage.QueryPromQL(ctx, `http_requests_total{status=~"5.."}`)

// Aggregations
storage.QueryPromQL(ctx, `sum(http_requests_total)`)
storage.QueryPromQL(ctx, `avg(http_requests_total) by (method)`)
storage.QueryPromQL(ctx, `count(http_requests_total) without (instance)`)

// Functions
storage.QueryPromQL(ctx, `abs(temperature)`)
storage.QueryPromQL(ctx, `ceil(latency_seconds)`)
storage.QueryPromQL(ctx, `clamp(cpu_usage, 0, 100)`)

// Binary operators
storage.QueryPromQL(ctx, `http_requests_total / http_requests_duration_seconds`)
storage.QueryPromQL(ctx, `memory_used / memory_total * 100`)
```

## Thread Safety

SpitDB is fully thread-safe. You can safely:
- Write samples from multiple goroutines
- Query while writing
- Share a single storage instance across your application

## Performance

SpitDB is optimized for:
- Fast writes (O(1) per sample via hash map)
- Efficient memory usage (single value per series)
- Quick queries (leverages Prometheus's optimized PromQL engine)

Typical performance on modern hardware:
- Write: ~1M samples/second
- Query: Sub-millisecond for most aggregations
- Memory: ~200 bytes per unique series

## Example: Metric Aggregator

```go
// Collect metrics from multiple Kubernetes nodes
for _, node := range nodes {
    samples := fetchMetricsFromKubelet(node)
    storage.WriteSamples(ctx, samples)
}

// Aggregate and expose combined metrics
results, _ := storage.QueryPromQL(ctx, `sum(container_cpu_usage_seconds_total) by (namespace)`)

// Expose as Prometheus metrics
for _, r := range results {
    promRegistry.MustRegister(prometheus.NewGaugeFunc(
        prometheus.GaugeOpts{Name: "namespace_cpu_total", ConstLabels: r.Labels},
        func() float64 { return r.Value },
    ))
}
```

## License

Apache License 2.0
