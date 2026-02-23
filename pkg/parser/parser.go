package parser

import (
	"errors"
	"io"

	"github.com/bdw617/spitdb/pkg/db"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

// ParsePrometheus parses Prometheus exposition format into SpitDB samples.
// extra labels are merged into every sample, useful for adding identifying
// labels like node="host-1" when ingesting from multiple sources.
func ParsePrometheus(r io.Reader, extra labels.Labels) []db.Sample {
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
			extra.Range(func(l labels.Label) {
				b.Add(l.Name, l.Value)
			})
			b.Sort()
			samples = append(samples, db.Sample{
				Labels: b.Labels(),
				Value:  float64(s.Value),
			})
		}
	}
	return samples
}
