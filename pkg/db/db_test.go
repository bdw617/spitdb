package db

import (
	"context"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

func TestStorage_WriteSamples(t *testing.T) {
	storage := New()
	ctx := context.Background()

	samples := []Sample{
		{
			Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "test_metric", "label1": "value1"}),
			Value:  42.5,
		},
		{
			Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "test_metric", "label1": "value2"}),
			Value:  100.0,
		},
	}

	err := storage.WriteSamples(ctx, samples)
	if err != nil {
		t.Fatalf("failed to write samples: %v", err)
	}

	if storage.Len() != 2 {
		t.Errorf("expected 2 series, got %d", storage.Len())
	}
}

func TestStorage_WriteSamples_Deduplication(t *testing.T) {
	storage := New()
	ctx := context.Background()

	// Write same series twice - last write wins
	sample1 := Sample{
		Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "test_metric", "label1": "value1"}),
		Value:  1.0,
	}
	sample2 := Sample{
		Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "test_metric", "label1": "value1"}),
		Value:  2.0,
	}

	_ = storage.WriteSamples(ctx, []Sample{sample1})
	_ = storage.WriteSamples(ctx, []Sample{sample2})

	if storage.Len() != 1 {
		t.Errorf("expected 1 series after deduplication, got %d", storage.Len())
	}
}

func TestStorage_Clear(t *testing.T) {
	storage := New()
	ctx := context.Background()

	samples := []Sample{
		{
			Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "test_metric"}),
			Value:  42.5,
		},
	}

	_ = storage.WriteSamples(ctx, samples)
	if storage.Len() != 1 {
		t.Fatalf("expected 1 series, got %d", storage.Len())
	}

	storage.Clear()
	if storage.Len() != 0 {
		t.Errorf("expected 0 series after clear, got %d", storage.Len())
	}
}

func TestStorage_Queryable(t *testing.T) {
	storage := New()
	ctx := context.Background()

	samples := []Sample{
		{
			Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "cpu_usage", "host": "server1"}),
			Value:  10.0,
		},
		{
			Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "cpu_usage", "host": "server2"}),
			Value:  20.0,
		},
	}

	_ = storage.WriteSamples(ctx, samples)

	querier, err := storage.Querier(0, 0)
	if err != nil {
		t.Fatalf("failed to get querier: %v", err)
	}
	defer querier.Close()

	// Test LabelNames
	names, _, err := querier.LabelNames(ctx, nil)
	if err != nil {
		t.Fatalf("LabelNames failed: %v", err)
	}
	if len(names) != 2 { // __name__ and host
		t.Errorf("expected 2 label names, got %d", len(names))
	}

	// Test LabelValues
	values, _, err := querier.LabelValues(ctx, "host", nil)
	if err != nil {
		t.Fatalf("LabelValues failed: %v", err)
	}
	if len(values) != 2 {
		t.Errorf("expected 2 host values, got %d", len(values))
	}
}
