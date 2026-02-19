package parser

import (
	"os"
	"strings"
	"testing"

	"github.com/bdw617/spitdb/pkg/db"
)

// find returns the first sample whose labels match all key/value pairs, or nil.
func find(samples []db.Sample, matchers map[string]string) *db.Sample {
	for i := range samples {
		match := true
		for k, v := range matchers {
			if samples[i].Labels.Get(k) != v {
				match = false
				break
			}
		}
		if match {
			return &samples[i]
		}
	}
	return nil
}

const counterInput = `# HELP http_requests_total Total HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="GET",code="200"} 100
http_requests_total{method="POST",code="201"} 50
`

func TestParsePrometheus_Counter(t *testing.T) {
	samples := ParsePrometheus(strings.NewReader(counterInput))
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}

	s := find(samples, map[string]string{"__name__": "http_requests_total", "method": "GET", "code": "200"})
	if s == nil {
		t.Fatal("GET/200 sample not found")
	}
	if s.Value != 100 {
		t.Errorf("expected value 100, got %v", s.Value)
	}

	s = find(samples, map[string]string{"__name__": "http_requests_total", "method": "POST", "code": "201"})
	if s == nil {
		t.Fatal("POST/201 sample not found")
	}
	if s.Value != 50 {
		t.Errorf("expected value 50, got %v", s.Value)
	}
}

const histogramInput = `# HELP rpc_duration_seconds RPC duration.
# TYPE rpc_duration_seconds histogram
rpc_duration_seconds_bucket{le="0.1"} 5
rpc_duration_seconds_bucket{le="0.5"} 12
rpc_duration_seconds_bucket{le="+Inf"} 15
rpc_duration_seconds_sum 4.2
rpc_duration_seconds_count 15
`

func TestParsePrometheus_Histogram(t *testing.T) {
	samples := ParsePrometheus(strings.NewReader(histogramInput))
	// 3 buckets + _sum + _count
	if len(samples) != 5 {
		t.Fatalf("expected 5 samples, got %d", len(samples))
	}

	cases := []struct {
		matchers map[string]string
		value    float64
	}{
		{map[string]string{"__name__": "rpc_duration_seconds_bucket", "le": "0.1"}, 5},
		{map[string]string{"__name__": "rpc_duration_seconds_bucket", "le": "0.5"}, 12},
		{map[string]string{"__name__": "rpc_duration_seconds_bucket", "le": "+Inf"}, 15},
		{map[string]string{"__name__": "rpc_duration_seconds_sum"}, 4.2},
		{map[string]string{"__name__": "rpc_duration_seconds_count"}, 15},
	}
	for _, c := range cases {
		s := find(samples, c.matchers)
		if s == nil {
			t.Errorf("sample not found: %v", c.matchers)
			continue
		}
		if s.Value != c.value {
			t.Errorf("matchers %v: expected %v, got %v", c.matchers, c.value, s.Value)
		}
	}
}

const summaryInput = `# HELP go_gc_duration_seconds GC pause duration.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 0.000123
go_gc_duration_seconds{quantile="0.5"} 0.000456
go_gc_duration_seconds{quantile="1"} 0.000789
go_gc_duration_seconds_sum 0.123
go_gc_duration_seconds_count 42
`

func TestParsePrometheus_Summary(t *testing.T) {
	samples := ParsePrometheus(strings.NewReader(summaryInput))
	// 3 quantiles + _sum + _count
	if len(samples) != 5 {
		t.Fatalf("expected 5 samples, got %d", len(samples))
	}

	s := find(samples, map[string]string{"__name__": "go_gc_duration_seconds", "quantile": "0.5"})
	if s == nil {
		t.Fatal("quantile=0.5 sample not found")
	}
	if s.Value != 0.000456 {
		t.Errorf("expected 0.000456, got %v", s.Value)
	}

	s = find(samples, map[string]string{"__name__": "go_gc_duration_seconds_count"})
	if s == nil {
		t.Fatal("_count sample not found")
	}
	if s.Value != 42 {
		t.Errorf("expected count 42, got %v", s.Value)
	}
}

func TestParsePrometheus_Empty(t *testing.T) {
	samples := ParsePrometheus(strings.NewReader(""))
	if len(samples) != 0 {
		t.Errorf("expected 0 samples, got %d", len(samples))
	}
}

func TestParsePrometheus_KubeletFile(t *testing.T) {
	f, err := os.Open("../../testdata/kubelet_metrics.txt")
	if err != nil {
		t.Skipf("testdata not available: %v", err)
	}
	defer f.Close()

	samples := ParsePrometheus(f)
	if len(samples) == 0 {
		t.Fatal("expected samples from kubelet metrics file, got none")
	}

	// Spot-check known metrics from the file.
	cases := []struct {
		matchers map[string]string
		value    float64
	}{
		{map[string]string{"__name__": "aggregator_discovery_aggregation_count_total"}, 0},
		{map[string]string{"__name__": "apiserver_delegated_authn_request_total", "code": "201"}, 91},
		{map[string]string{"__name__": "apiserver_client_certificate_expiration_seconds_bucket", "le": "+Inf"}, 17},
		{map[string]string{"__name__": "apiserver_client_certificate_expiration_seconds_count"}, 17},
	}
	for _, c := range cases {
		s := find(samples, c.matchers)
		if s == nil {
			t.Errorf("sample not found: %v", c.matchers)
			continue
		}
		if s.Value != c.value {
			t.Errorf("matchers %v: expected %v, got %v", c.matchers, c.value, s.Value)
		}
	}
}
