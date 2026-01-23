package spitdb

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
}

func TestStorage_QueryPromQL(t *testing.T) {
	storage := New()
	ctx := context.Background()

	// Write test data
	samples := []Sample{
		{
			Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "cpu_usage", "namespace": "default", "pod": "pod-a"}),
			Value:  10.0,
		},
		{
			Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "cpu_usage", "namespace": "default", "pod": "pod-b"}),
			Value:  20.0,
		},
		{
			Labels: labels.FromMap(map[string]string{string(model.MetricNameLabel): "cpu_usage", "namespace": "kube-system", "pod": "pod-c"}),
			Value:  30.0,
		},
	}

	if err := storage.WriteSamples(ctx, samples); err != nil {
		t.Fatalf("failed to write samples: %v", err)
	}

	// Test sum aggregation
	results, err := storage.QueryPromQL(ctx, `sum(cpu_usage)`)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Value != 60.0 {
		t.Errorf("expected sum 60.0, got %v", results[0].Value)
	}
}

func TestNew(t *testing.T) {
	storage := New()
	if storage == nil {
		t.Fatal("New returned nil")
	}
	if storage.Storage == nil {
		t.Fatal("Storage.Storage is nil")
	}
	if storage.engine == nil {
		t.Fatal("Storage.engine is nil")
	}
}
