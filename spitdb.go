// Package spitdb provides a Single Point In Time Database - an in-memory
// time-series storage optimized for point-in-time metric snapshots with full
// PromQL query support.
//
// SpitDB is designed for scenarios where you need to:
//   - Aggregate metrics from multiple sources on-demand
//   - Execute PromQL queries without persistent storage
//   - Build lightweight metric proxies or aggregators
//
// For more control, use the sub-packages directly:
//   - github.com/bdw617/spitdb/pkg/db - Storage implementation
//   - github.com/bdw617/spitdb/pkg/query - PromQL query engine
package spitdb

import (
	"context"

	"github.com/bdw617/spitdb/pkg/db"
	"github.com/bdw617/spitdb/pkg/query"
)

// Storage combines the in-memory database with a PromQL query engine
// for convenient single-package usage.
type Storage struct {
	*db.Storage
	engine *query.Engine
}

// Sample is an alias for db.Sample for convenience.
type Sample = db.Sample

// QueryResult is an alias for query.Result for convenience.
type QueryResult = query.Result

// New creates a new Storage with default settings.
func New() *Storage {
	return &Storage{
		Storage: db.New(),
		engine:  query.NewEngine(nil),
	}
}

// NewWithOpts creates a new Storage with custom query engine options.
func NewWithOpts(opts *query.EngineOpts) *Storage {
	return &Storage{
		Storage: db.New(),
		engine:  query.NewEngine(opts),
	}
}

// QueryPromQL executes a PromQL query and returns the results.
// This supports full PromQL including aggregations, functions, etc.
func (s *Storage) QueryPromQL(ctx context.Context, queryStr string) ([]QueryResult, error) {
	return s.engine.Query(ctx, s.Storage, queryStr)
}
