package spitdb_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/bdw617/spitdb"
	"github.com/bdw617/spitdb/pkg/parser"
	"github.com/prometheus/prometheus/model/labels"
)

func loadTestdata(b *testing.B) []byte {
	b.Helper()
	f, err := os.Open("testdata/kubelet_metrics.txt")
	if err != nil {
		b.Skipf("testdata not available: %v", err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		b.Fatal(err)
	}
	return data
}

// BenchmarkIngest measures parse + storage write throughput across different
// fleet sizes. Each iteration parses one host's metrics and writes to storage.
func BenchmarkIngest(b *testing.B) {
	data := loadTestdata(b)
	ctx := context.Background()

	b.Run("BenchmarkBigIngest", func(b *testing.B) {
		store := spitdb.New()
		for i := range b.N {
			samples := parser.ParsePrometheus(bytes.NewReader(data), labels.FromMap(map[string]string{"node": fmt.Sprintf("node-%d", i+1)}))
			_ = store.WriteSamples(ctx, samples)
		}
		b.StopTimer()

		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		b.ReportMetric(float64(ms.HeapAlloc), "store-bytes")

		results, err := store.QueryPromQL(ctx, `count(authentication_token_cache_request_total) by (status)`)
		if err != nil {
			b.Fatalf("query failed: %v", err)
		}
		for _, r := range results {
			b.Logf("hostcount=%d status=%s: %.0f", b.N, r.Labels["status"], r.Value)
		}
	})

}
