// Package db provides an in-memory time-series storage that keeps only the latest
// value per metric series. It implements the Prometheus storage.Queryable interface
// for compatibility with PromQL.
package db

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/util/annotations"
)

// Storage is an in-memory storage that keeps only the latest value per series.
// It implements storage.Queryable to work with the PromQL engine.
//
// This is ideal for scenarios where you only need the current state of metrics,
// such as on-demand metric aggregation or point-in-time snapshots.
type Storage struct {
	mu     sync.RWMutex
	series map[uint64]*memorySeries // hash -> series
}

// memorySeries holds a single series with its latest value.
type memorySeries struct {
	labels labels.Labels
	value  float64
}

// New creates a new in-memory storage.
func New() *Storage {
	return &Storage{
		series: make(map[uint64]*memorySeries),
	}
}

// Sample represents a single metric sample with labels and a float64 value.
type Sample struct {
	Labels labels.Labels
	Value  float64
}

// WriteSamples writes samples to the storage. For duplicate series (same label set),
// the last write wins. This is thread-safe.
func (s *Storage) WriteSamples(ctx context.Context, samples []Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sample := range samples {
		hash := sample.Labels.Hash()
		s.series[hash] = &memorySeries{
			labels: sample.Labels,
			value:  sample.Value,
		}
	}

	return nil
}

// Len returns the number of unique series in the storage.
func (s *Storage) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.series)
}

// Clear removes all series from the storage.
func (s *Storage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.series = make(map[uint64]*memorySeries)
}

// Querier returns a new querier for the storage.
// This implements storage.Queryable.
func (s *Storage) Querier(mint, maxt int64) (storage.Querier, error) {
	return &memoryQuerier{storage: s}, nil
}

// memoryQuerier implements storage.Querier.
type memoryQuerier struct {
	storage *Storage
}

// Select returns a SeriesSet that matches the given matchers.
// Since we only store the latest value per series, we always return it
// regardless of the time range (the PromQL engine handles staleness).
func (q *memoryQuerier) Select(ctx context.Context, sortSeries bool, hints *storage.SelectHints, matchers ...*labels.Matcher) storage.SeriesSet {
	q.storage.mu.RLock()
	defer q.storage.mu.RUnlock()

	var matched []*memorySeries
	for _, series := range q.storage.series {
		if matchesAll(series.labels, matchers) {
			matched = append(matched, series)
		}
	}

	return &memorySeriesSet{
		series:    matched,
		idx:       -1,
		queryTime: time.Now().UnixMilli(),
	}
}

// LabelValues returns all values for a given label name.
func (q *memoryQuerier) LabelValues(ctx context.Context, name string, hints *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	q.storage.mu.RLock()
	defer q.storage.mu.RUnlock()

	values := make(map[string]struct{})
	for _, series := range q.storage.series {
		if len(matchers) > 0 && !matchesAll(series.labels, matchers) {
			continue
		}
		if v := series.labels.Get(name); v != "" {
			values[v] = struct{}{}
		}
	}

	result := make([]string, 0, len(values))
	for v := range values {
		result = append(result, v)
	}
	return result, nil, nil
}

// LabelNames returns all label names.
func (q *memoryQuerier) LabelNames(ctx context.Context, hints *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	q.storage.mu.RLock()
	defer q.storage.mu.RUnlock()

	names := make(map[string]struct{})
	for _, series := range q.storage.series {
		if len(matchers) > 0 && !matchesAll(series.labels, matchers) {
			continue
		}
		series.labels.Range(func(l labels.Label) {
			names[l.Name] = struct{}{}
		})
	}

	result := make([]string, 0, len(names))
	for n := range names {
		result = append(result, n)
	}
	return result, nil, nil
}

// Close releases resources.
func (q *memoryQuerier) Close() error {
	return nil
}

// matchesAll checks if all matchers match the given labels.
func matchesAll(lset labels.Labels, matchers []*labels.Matcher) bool {
	for _, m := range matchers {
		if !m.Matches(lset.Get(m.Name)) {
			return false
		}
	}
	return true
}

// memorySeriesSet implements storage.SeriesSet.
type memorySeriesSet struct {
	series    []*memorySeries
	idx       int
	queryTime int64
}

func (s *memorySeriesSet) Next() bool {
	s.idx++
	return s.idx < len(s.series)
}

func (s *memorySeriesSet) At() storage.Series {
	return &memorySingleSeries{series: s.series[s.idx], queryTime: s.queryTime}
}

func (s *memorySeriesSet) Err() error {
	return nil
}

func (s *memorySeriesSet) Warnings() annotations.Annotations {
	return nil
}

// memorySingleSeries implements storage.Series.
type memorySingleSeries struct {
	series    *memorySeries
	queryTime int64
}

func (s *memorySingleSeries) Labels() labels.Labels {
	return s.series.labels
}

func (s *memorySingleSeries) Iterator(it chunkenc.Iterator) chunkenc.Iterator {
	return &singleValueIterator{
		ts:    s.queryTime,
		value: s.series.value,
	}
}

// singleValueIterator implements chunkenc.Iterator for a single sample.
type singleValueIterator struct {
	ts       int64
	value    float64
	consumed bool // true after the single value has been returned via Next/Seek
}

func (it *singleValueIterator) Next() chunkenc.ValueType {
	if it.consumed {
		return chunkenc.ValNone
	}
	it.consumed = true
	return chunkenc.ValFloat
}

func (it *singleValueIterator) Seek(_ int64) chunkenc.ValueType {
	it.consumed = true
	return chunkenc.ValFloat
}

func (it *singleValueIterator) At() (int64, float64) {
	return it.ts, it.value
}

func (it *singleValueIterator) AtHistogram(*histogram.Histogram) (int64, *histogram.Histogram) {
	return 0, nil
}

func (it *singleValueIterator) AtFloatHistogram(*histogram.FloatHistogram) (int64, *histogram.FloatHistogram) {
	return 0, nil
}

func (it *singleValueIterator) AtT() int64 {
	return it.ts
}

func (it *singleValueIterator) Err() error {
	return nil
}
