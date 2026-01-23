package query

import (
	"context"
	"testing"

	"github.com/bdw617/spitdb/pkg/db"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

func TestEngine_Query(t *testing.T) {
	storage := db.New()
	engine := NewEngine(nil)
	ctx := context.Background()

	// Write test data
	samples := []db.Sample{
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

	tests := []struct {
		name          string
		query         string
		expectedLen   int
		expectedValue float64
	}{
		{
			name:        "simple selector",
			query:       `cpu_usage`,
			expectedLen: 3,
		},
		{
			name:        "selector with label matcher",
			query:       `cpu_usage{namespace="default"}`,
			expectedLen: 2,
		},
		{
			name:          "sum aggregation",
			query:         `sum(cpu_usage)`,
			expectedLen:   1,
			expectedValue: 60.0, // 10 + 20 + 30
		},
		{
			name:        "sum by namespace",
			query:       `sum(cpu_usage) by (namespace)`,
			expectedLen: 2, // default and kube-system
		},
		{
			name:          "avg",
			query:         `avg(cpu_usage)`,
			expectedLen:   1,
			expectedValue: 20.0, // (10 + 20 + 30) / 3
		},
		{
			name:          "count",
			query:         `count(cpu_usage)`,
			expectedLen:   1,
			expectedValue: 3.0,
		},
		{
			name:          "max",
			query:         `max(cpu_usage)`,
			expectedLen:   1,
			expectedValue: 30.0,
		},
		{
			name:          "min",
			query:         `min(cpu_usage)`,
			expectedLen:   1,
			expectedValue: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := engine.Query(ctx, storage, tt.query)
			if err != nil {
				t.Fatalf("query failed: %v", err)
			}

			if len(results) != tt.expectedLen {
				t.Errorf("expected %d results, got %d", tt.expectedLen, len(results))
			}

			if tt.expectedValue != 0 && len(results) > 0 {
				if results[0].Value != tt.expectedValue {
					t.Errorf("expected value %v, got %v", tt.expectedValue, results[0].Value)
				}
			}
		})
	}
}

func TestEngine_Query_NoResults(t *testing.T) {
	storage := db.New()
	engine := NewEngine(nil)
	ctx := context.Background()

	results, err := engine.Query(ctx, storage, `nonexistent_metric`)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestNewEngine_CustomOpts(t *testing.T) {
	engine := NewEngine(&EngineOpts{
		MaxSamples: 1000,
	})

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
}
