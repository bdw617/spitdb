// Kubelet Problem Checker demonstrates SpitDB's core use case: ingest a raw
// kubelet /metrics scrape (~2500 lines), run curated PromQL queries that
// surface only problems, and output a much smaller human-readable result
// in Prometheus exposition format.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"go.yaml.in/yaml/v2"

	"github.com/bdw617/spitdb"
	"github.com/bdw617/spitdb/pkg/db"
	"github.com/bdw617/spitdb/pkg/parser"
)

type queryDef struct {
	Name  string `yaml:"name"`
	Help  string `yaml:"help"`
	Query string `yaml:"query"`
}

type queryConfig struct {
	Queries []queryDef `yaml:"queries"`
}

func main() {
	metricsFile := flag.String("metrics", "", "path to kubelet metrics file (required)")
	queriesFile := flag.String("queries", "", "path to queries YAML (default: queries.yaml next to binary)")
	flag.Parse()

	if *metricsFile == "" {
		fmt.Fprintln(os.Stderr, "usage: kubelet-checker --metrics <file> [--queries <file>]")
		os.Exit(1)
	}

	if *queriesFile == "" {
		exe, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		*queriesFile = filepath.Join(filepath.Dir(exe), "queries.yaml")
	}

	samples := exampleGetSamplesfromFile(*metricsFile)

	// Load into SpitDB.
	storage := spitdb.New()
	ctx := context.Background()
	if err := storage.WriteSamples(ctx, samples); err != nil {
		log.Fatalf("writing samples: %v", err)
	}

	// Load query config.
	queriesData, err := os.ReadFile(*queriesFile)
	if err != nil {
		log.Fatalf("reading queries: %v", err)
	}
	var cfg queryConfig
	if err := yaml.Unmarshal(queriesData, &cfg); err != nil {
		log.Fatalf("parsing queries YAML: %v", err)
	}

	fmt.Print(generateOutput(ctx, cfg, storage))
}

// Example code to read metrics and convert to samples.
func exampleGetSamplesfromFile(metricsFile string) []db.Sample { // for relative path in queries.yaml
	f, err := os.Open(metricsFile)
	if err != nil {
		log.Fatalf("opening metrics: %v", err)
	}
	defer f.Close()
	samples := parser.ParsePrometheus(f)
	return samples
}

// Example code to convert run a bunch of prometheus queries against the storage, and output results in Prometheus exposition format.
func generateOutput(ctx context.Context, cfg queryConfig, storage *spitdb.Storage) string {
	var sb strings.Builder
	for _, qd := range cfg.Queries {
		results, err := storage.QueryPromQL(ctx, qd.Query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "# ERROR %s: %v\n", qd.Name, err)
			continue
		}
		if len(results) == 0 {
			continue
		}

		fmt.Fprintf(&sb, "# HELP %s %s\n", qd.Name, qd.Help)
		fmt.Fprintf(&sb, "# TYPE %s gauge\n", qd.Name)
		for _, r := range results {
			fmt.Fprintf(&sb, "%s%s %s\n", qd.Name, formatLabels(r.Labels), model.SampleValue(r.Value).String())
		}
	}
	return sb.String()
}

// formatLabels renders a label map as {k="v",...} sorted by key.
// The query engine already strips __name__, so no filtering needed.
func formatLabels(m map[string]string) string {
	ls := labels.FromMap(m)
	if ls.Len() == 0 {
		return ""
	}
	return ls.String()
}
