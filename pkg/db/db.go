package db

import (
	"context"
	"sync"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/util/annotations"
)

const shardCount = 128

type Storage struct {
	shards [shardCount]*shard
}

type shard struct {
	mu     sync.RWMutex
	series map[uint64]Sample
}

// memoryQuerier implements storage.Querier.
type memoryQuerier struct {
	storage   *Storage
	queryTime int64 // query time in milliseconds, from maxt
}

func New() *Storage {
	s := &Storage{}
	for i := 0; i < shardCount; i++ {
		s.shards[i] = &shard{series: make(map[uint64]Sample)}
	}
	return s
}

type Sample struct {
	Hash   uint64 // Store hash to avoid re-calculating
	Labels labels.Labels
	Value  float64
}

func (s *Storage) WriteSamples(ctx context.Context, samples []Sample) error {
	for _, sample := range samples {
		h := sample.Labels.Hash()
		shard := s.shards[h%shardCount]

		shard.mu.Lock()
		shard.series[h] = sample
		shard.mu.Unlock()
	}
	return nil
}

func (s *memoryQuerier) Select(ctx context.Context, sortSeries bool, hints *storage.SelectHints, matchers ...*labels.Matcher) storage.SeriesSet {
	var matched []Sample
	for i := 0; i < shardCount; i++ {
		shard := s.storage.shards[i]
		shard.mu.RLock()
		for _, series := range shard.series {
			if matchesAll(series.Labels, matchers) {
				matched = append(matched, series)
			}
		}
		shard.mu.RUnlock()
	}
	return &memorySeriesSet{series: matched, idx: -1, queryTime: s.queryTime}
}

func (s *Storage) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.Lock()
		shard.series = make(map[uint64]Sample)
		shard.mu.Unlock()
	}
}

// Len returns the number of unique series in the storage.
func (s *Storage) Len() int {
	total := 0
	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.Lock()
		total += len(shard.series)
		shard.mu.Unlock()
	}
	return total
}

// Querier returns a new querier for the storage.
// This implements storage.Queryable.
func (s *Storage) Querier(mint, maxt int64) (storage.Querier, error) {
	return &memoryQuerier{storage: s, queryTime: maxt}, nil
}

// LabelValues returns all values for a given label name.
func (q *memoryQuerier) LabelValues(ctx context.Context, name string, hints *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	seen := make(map[string]struct{})

	for i := 0; i < shardCount; i++ {
		shard := q.storage.shards[i]
		shard.mu.RLock()
		defer shard.mu.RUnlock()
		for _, series := range shard.series {
			if len(matchers) > 0 && !matchesAll(series.Labels, matchers) {
				continue
			}
			if v := series.Labels.Get(name); v != "" {
				seen[v] = struct{}{}
			}
		}
	}
	return dedupe(seen), nil, nil
}

func dedupe(seen map[string]struct{}) []string {
	result := make([]string, 0, len(seen))
	for v := range seen {
		result = append(result, v)
	}
	return result
}

// LabelNames returns all label names.
func (q *memoryQuerier) LabelNames(ctx context.Context, hints *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	seen := make(map[string]struct{})

	for i := 0; i < shardCount; i++ {
		shard := q.storage.shards[i]
		shard.mu.RLock()
		for _, series := range shard.series {
			if len(matchers) > 0 && !matchesAll(series.Labels, matchers) {
				continue
			}
			series.Labels.Range(func(l labels.Label) {
				seen[l.Name] = struct{}{}
			})
		}
		shard.mu.RUnlock()
	}
	return dedupe(seen), nil, nil
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
	series    []Sample
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
	series    Sample
	queryTime int64
}

func (s *memorySingleSeries) Labels() labels.Labels {
	return s.series.Labels
}

func (s *memorySingleSeries) Iterator(it chunkenc.Iterator) chunkenc.Iterator {
	return &singleValueIterator{
		ts:    s.queryTime,
		value: s.series.Value,
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
