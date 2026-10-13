package convert

import (
	"fmt"
	"sort"

	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

// NewMimirSeriesProto creates a new proto for use in unmarshalling data from
// USTable files.
func NewMimirSeriesProto() ProtoUnmarshaler {
	return &mimirpb.TimeSeries{}
}

// MimirSeries satisfies the storage.Series interface for writing out to Blocks.
type MimirSeries struct {
	l labels.Labels
	s []mimirpb.Sample
}

func NewMimirSeries(l labels.Labels, samples []mimirpb.Sample) *MimirSeries {
	return &MimirSeries{l, samples}
}

func (s *MimirSeries) Labels() labels.Labels {
	return s.l
}

type mimirSeriesIterator struct {
	idx int64
	s   *MimirSeries
	err error
}

func (s *MimirSeries) Iterator(_ chunkenc.Iterator) chunkenc.Iterator {
	return &mimirSeriesIterator{-1, s, nil}
}

func (i *mimirSeriesIterator) len() int64 {
	return int64(len(i.s.s))
}

func (i *mimirSeriesIterator) Next() chunkenc.ValueType {
	if i.err != nil {
		return chunkenc.ValNone
	}
	i.idx++
	if i.idx >= i.len() {
		i.idx = i.len()
		return chunkenc.ValNone
	}
	return chunkenc.ValFloat
}

// Seek looks for the next available sample whose timestamp is >= the requested
// ts. See https://github.com/prometheus/prometheus/blob/main/tsdb/chunkenc/chunk.go#L79-L83
// for details on Seek behavior.
func (i *mimirSeriesIterator) Seek(ts int64) chunkenc.ValueType { //nolint: govet
	if i.err != nil {
		return chunkenc.ValNone
	}
	// If current position timestamp is >= ts, Seek has no effect. This check only makes sense to perform if there is a
	// current position (i.idx > -1).
	if i.idx > -1 {
		curT := i.AtT()
		if curT >= ts {
			return chunkenc.ValFloat
		}
	}
	// Early-exit test to see if there is no sample >= the requested ts.
	if i.s.s[i.len()-1].TimestampMs < ts {
		i.err = fmt.Errorf("no samples with timestamp >= %d", ts)
		return chunkenc.ValNone
	}

	i.idx = int64(sort.Search(int(i.len()-1), func(idx int) bool {
		curT := i.s.s[idx].TimestampMs
		return curT >= ts
	}))
	return chunkenc.ValFloat
}

func (i *mimirSeriesIterator) At() (ts int64, v float64) {
	return i.s.s[i.idx].TimestampMs, i.s.s[i.idx].Value
}

func (i *mimirSeriesIterator) AtT() int64 {
	return i.s.s[i.idx].TimestampMs
}

func (i *mimirSeriesIterator) AtST() int64 {
	return 0
}

func (i *mimirSeriesIterator) AtFloatHistogram(_ *histogram.FloatHistogram) (int64, *histogram.FloatHistogram) {
	return 0, nil
}

func (i *mimirSeriesIterator) AtHistogram(_ *histogram.Histogram) (int64, *histogram.Histogram) {
	return 0, nil
}

func (i *mimirSeriesIterator) Err() error {
	return i.err
}
