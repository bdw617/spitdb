// Package query provides PromQL query execution against any Prometheus-compatible
// storage that implements the storage.Queryable interface.
package query

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/schema"
	"github.com/prometheus/prometheus/storage"
)

// Engine wraps a PromQL engine for executing queries against any storage.Queryable.
type Engine struct {
	engine *promql.Engine
}

// EngineOpts configures the query engine.
type EngineOpts struct {
	// MaxSamples is the maximum number of samples a query can load into memory.
	// Default: 50,000,000
	MaxSamples int
	// Timeout is the maximum time a query can run before being cancelled.
	// Default: 2 minutes
	Timeout time.Duration
}

// NewEngine creates a new PromQL query engine with the given options.
// If opts is nil, default options are used.
func NewEngine(opts *EngineOpts) *Engine {
	if opts == nil {
		opts = &EngineOpts{}
	}
	if opts.MaxSamples == 0 {
		opts.MaxSamples = 50000000
	}
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Minute
	}

	return &Engine{
		engine: promql.NewEngine(promql.EngineOpts{
			MaxSamples: opts.MaxSamples,
			Timeout:    opts.Timeout,
		}),
	}
}

// Result represents a single query result with labels and value.
type Result struct {
	Labels map[string]string
	Value  float64
}

// Query executes a PromQL instant query against the given queryable storage
// and returns the results.
//
// This supports full PromQL including:
//   - Label selectors: metric{label="value"}
//   - Aggregations: sum, avg, count, min, max, etc.
//   - Functions: rate, increase, histogram_quantile, etc.
//   - Binary operators: +, -, *, /, etc.
//   - Group modifiers: by, without
func (e *Engine) Query(ctx context.Context, queryable storage.Queryable, query string) ([]Result, error) {
	now := time.Now()
	q, err := e.engine.NewInstantQuery(ctx, queryable, nil, query, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer q.Close()

	res := q.Exec(ctx)
	if res.Err != nil {
		return nil, fmt.Errorf("query execution failed: %w", res.Err)
	}

	var results []Result
	switch v := res.Value.(type) {
	case promql.Vector:
		for _, sample := range v {
			results = append(results, Result{
				Labels: sample.Metric.DropReserved(schema.IsMetadataLabel).Map(),
				Value:  sample.F,
			})
		}
	case promql.Scalar:
		results = append(results, Result{
			Labels: nil,
			Value:  v.V,
		})
	case promql.Matrix:
		// For matrix results, get the latest value from each series
		for _, series := range v {
			if len(series.Floats) > 0 {
				lastPoint := series.Floats[len(series.Floats)-1]
				results = append(results, Result{
					Labels: series.Metric.DropReserved(schema.IsMetadataLabel).Map(),
					Value:  lastPoint.F,
				})
			}
		}
	}
	return results, nil
}

// QueryAt executes a PromQL instant query at a specific timestamp.
func (e *Engine) QueryAt(ctx context.Context, queryable storage.Queryable, query string, ts time.Time) ([]Result, error) {
	q, err := e.engine.NewInstantQuery(ctx, queryable, nil, query, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer q.Close()

	res := q.Exec(ctx)
	if res.Err != nil {
		return nil, fmt.Errorf("query execution failed: %w", res.Err)
	}

	var results []Result
	switch v := res.Value.(type) {
	case promql.Vector:
		for _, sample := range v {
			results = append(results, Result{
				Labels: sample.Metric.DropReserved(schema.IsMetadataLabel).Map(),
				Value:  sample.F,
			})
		}
	case promql.Scalar:
		results = append(results, Result{
			Labels: nil,
			Value:  v.V,
		})
	case promql.Matrix:
		for _, series := range v {
			if len(series.Floats) > 0 {
				lastPoint := series.Floats[len(series.Floats)-1]
				results = append(results, Result{
					Labels: series.Metric.DropReserved(schema.IsMetadataLabel).Map(),
					Value:  lastPoint.F,
				})
			}
		}
	}
	return results, nil
}
