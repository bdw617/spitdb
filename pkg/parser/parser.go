package parser

import (
	"errors"
	"io"

	"github.com/bdw617/spitdb/pkg/db"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

// parseMetrics parses Prometheus exposition format into SpitDB samples.
func ParsePrometheus(r io.Reader) []db.Sample {
	// NOTE: can we tell the Decoder to only include a subset of __name__'s?
	dec := &expfmt.SampleDecoder{
		Dec:  expfmt.NewDecoder(r, expfmt.NewFormat(expfmt.TypeTextPlain)),
		Opts: &expfmt.DecodeOptions{Timestamp: model.Now()},
	}
	var (
		samples []db.Sample
		batch   model.Vector
		b       labels.ScratchBuilder
	)
	for {
		if err := dec.Decode(&batch); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		for _, s := range batch {
			b.Reset()
			for k, v := range s.Metric {
				b.Add(string(k), string(v))
			}
			b.Sort()
			samples = append(samples, db.Sample{
				Labels: b.Labels(),
				Value:  float64(s.Value),
			})
		}
	}
	return samples
}
