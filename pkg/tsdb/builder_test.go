package tsdb

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	log2 "github.com/grafana/mimir/pkg/util/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/index"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:gosec // Disable linter complaining about insecure random numbers. We don't use random numbers in security context here.
func TestTsdbBuilder(t *testing.T) {
	tmpDir := t.TempDir()

	builder, err := NewBuilder(tmpDir, Options{
		SymbolsBatchSize: 1000,
		SeriesBatchSize:  1000,
	})
	require.NoError(t, err)

	var seriesMu sync.Mutex
	series := map[string][]model.SamplePair{}

	minT := timestamp.FromTime(time.Now())

	const (
		concurrency         = 10
		seriesPerGoroutine  = 1000
		maxSamples          = 1000
		seriesMinTimeOffset = 1 * time.Hour
		minStep             = 1 * time.Second
		maxStep             = 1 * time.Minute
	)

	var wg sync.WaitGroup
	// Add some random series, concurrently
	for i := 0; i < concurrency; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			for j := 0; j < seriesPerGoroutine; j++ {
				labelsMap := map[string]string{
					"__name__": fmt.Sprintf("series_%d", j),
					"async":    fmt.Sprintf("%d", i),
					"__aaa__":  fmt.Sprintf("%d", seriesPerGoroutine-j), // sorts before __name__
				}

				stepMillis := minStep.Milliseconds() + rand.Int63n(maxStep.Milliseconds()-minStep.Milliseconds())
				count := 1 + rand.Intn(maxSamples)
				seriesMinT := minT + rand.Int63n(seriesMinTimeOffset.Milliseconds())

				samples := make([]model.SamplePair, 0, count)
				for s := 0; s < count; s++ {
					samples = append(samples, model.SamplePair{
						Timestamp: model.Time(seriesMinT) + model.Time(int64(s)*stepMillis),
						Value:     model.SampleValue(s),
					})
				}

				lbls := labels.FromMap(labelsMap)
				seriesMu.Lock()
				series[lbls.String()] = samples
				seriesMu.Unlock()

				err1 := builder.AddSeriesWithSamples(lbls, newSamplesIterator(samples))
				if err1 != nil {
					require.NoError(t, err1)
				}
			}
		}(i)
	}

	wg.Wait()

	id, err := builder.FinishBlock(context.Background(), func(meta tsdb.BlockMeta) interface{} { return meta })
	require.NoError(t, err)

	verifyBlock(t, filepath.Join(tmpDir, id.String()), series)
}

//nolint:gosec // Disable linter complaining about insecure random numbers. We don't use random numbers in security context here.
func TestCreateBlock(t *testing.T) {
	series := []storage.Series(nil)
	seriesMap := map[string][]model.SamplePair{} // used for verification

	// Generate series.
	minT := timestamp.FromTime(time.Now())

	const (
		maxSeries           = 10000
		maxSamplesPerSeries = 1000
		seriesMinTimeOffset = 1 * time.Hour
		minStep             = 1 * time.Second
		maxStep             = 1 * time.Minute
	)

	for j := 0; j < maxSeries; j++ {
		labelsMap := map[string]string{
			"__name__": fmt.Sprintf("series_%d", j),
			"random":   fmt.Sprintf("%d", rand.Int()), // Some big number.
			"__aaa__":  fmt.Sprintf("%d", maxSeries),  // sorts before __name__
		}

		stepMillis := minStep.Milliseconds() + rand.Int63n(maxStep.Milliseconds()-minStep.Milliseconds())
		count := 1 + rand.Intn(maxSamplesPerSeries)
		seriesMinT := minT + rand.Int63n(seriesMinTimeOffset.Milliseconds())

		samples := make([]model.SamplePair, 0, count)
		for s := 0; s < count; s++ {
			samples = append(samples, model.SamplePair{
				Timestamp: model.Time(seriesMinT) + model.Time(int64(s)*stepMillis),
				Value:     model.SampleValue(s),
			})
		}

		lbls := labels.FromMap(labelsMap)
		series = append(series, newSeries(lbls, samples))
		seriesMap[lbls.String()] = samples
	}

	sort.Slice(series, func(i, j int) bool {
		return labels.Compare(series[i].Labels(), series[j].Labels()) < 0
	})

	dir := t.TempDir()

	id, err := CreateBlock(context.Background(), series, dir, func(meta tsdb.BlockMeta) interface{} { return meta })
	require.NoError(t, err)

	verifyBlock(t, filepath.Join(dir, id.String()), seriesMap)
}

func verifyBlock(t *testing.T, blockDir string, series map[string][]model.SamplePair) {
	b, err := tsdb.OpenBlock(log2.SlogFromGoKit(log.NewNopLogger()), blockDir, nil, nil)
	require.NoError(t, err)

	// Let's verify that all expected series and their samples are found in the block
	idx, err := b.Index()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = idx.Close()
	})

	chksReader, err := b.Chunks()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = chksReader.Close()
	})

	allK, allV := index.AllPostingsKey()
	p, err := idx.Postings(context.Background(), allK, allV)

	var stats tsdb.BlockStats
	minT, maxT := int64(math.MaxInt64), int64(0)

	prevLabels := labels.Labels{}
	require.NoError(t, err)
	for p.Next() {
		stats.NumSeries++

		ser := p.At()

		var lblsBuilder labels.ScratchBuilder
		var chks []chunks.Meta

		require.NoError(t, idx.Series(ser, &lblsBuilder, &chks))

		lbls := lblsBuilder.Labels()

		stats.NumChunks += uint64(len(chks))

		// Verify that series are sorted by labels.
		require.Negative(t, labels.Compare(prevLabels, lbls))
		prevLabels = lbls

		samples, ok := series[lbls.String()]
		assert.Truef(t, ok, "unexpected series in the index", lbls.String())

		for ix := 0; ix < len(chks); ix++ {
			chks[ix].Chunk, _, err = chksReader.ChunkOrIterable(chks[ix])
			require.NoError(t, err)
		}

		// verify chunks Min/Max times.
		require.LessOrEqual(t, chks[0].MinTime, chks[0].MaxTime)
		for ix := 1; ix < len(chks); ix++ {
			require.Less(t, chks[ix-1].MaxTime, chks[ix].MinTime)
			require.LessOrEqual(t, chks[ix].MinTime, chks[ix].MaxTime)
		}

		// Verify that all expected samples are found.
		cit := chunksIterator{
			chunks: chks,
			ix:     -1,
		}

		for len(samples) > 0 {
			require.True(t, cit.Next())
			stats.NumSamples++

			ts, v := cit.At()

			require.Equal(t, int64(samples[0].Timestamp), ts)
			require.Equal(t, float64(samples[0].Value), v)

			if ts < minT {
				minT = ts
			}
			if ts > maxT {
				maxT = ts
			}

			samples = samples[1:]
		}

		require.False(t, cit.Next())
		require.Nil(t, cit.Err())
		delete(series, lbls.String())
	}

	// Make sure all series were found in the block.
	require.Empty(t, series)

	meta := b.Meta()
	require.Equal(t, stats, meta.Stats)
	require.Equal(t, minT, meta.MinTime)
	require.Equal(t, maxT+1, meta.MaxTime) // block's maxT is exclusive

	// Check that block has only expected files in it.
	entries, err := os.ReadDir(blockDir)
	require.NoError(t, err)
	for _, e := range entries {
		switch {
		case e.Name() == "meta.json" && e.Type().IsRegular():
			// ok
		case e.Name() == "index" && e.Type().IsRegular():
			// ok
		case e.Name() == "chunks" && e.Type().IsDir():
			// ok
		default:
			assert.Failf(t, "unexpected dir entry", "name: %s, type: %s", e.Name(), e.Type())
		}
	}
}

type chunksIterator struct {
	chunks []chunks.Meta
	ix     int
	it     chunkenc.Iterator
}

func (c chunksIterator) Seek(_ int64) bool { //nolint: govet
	panic("implement me")
}

func (c chunksIterator) At() (ts int64, val float64) {
	return c.it.At()
}

func (c chunksIterator) Err() error {
	if c.it != nil {
		return c.it.Err()
	}
	return nil
}

func (c *chunksIterator) Next() bool {
	for {
		if c.it != nil {
			next := c.it.Next()
			if next != chunkenc.ValNone {
				return true
			}
			if c.it.Err() != nil {
				return false
			}
			c.it = nil
		}
		c.ix++
		if c.ix < len(c.chunks) {
			c.it = c.chunks[c.ix].Chunk.Iterator(nil)
		} else {
			return false
		}
	}
}

func newSamplesIterator(samples []model.SamplePair) *samplesIterator {
	return &samplesIterator{
		samples: samples,
		it:      -1,
	}
}

type samplesIterator struct {
	samples []model.SamplePair
	it      int
}

func (s *samplesIterator) Next() chunkenc.ValueType {
	s.it++
	if s.it < len(s.samples) {
		return chunkenc.ValFloat
	}
	return chunkenc.ValNone
}

func (s samplesIterator) Seek(int64) chunkenc.ValueType { // nolint: govet
	panic("not implemented")
}

func (s samplesIterator) At() (ts int64, val float64) {
	return int64(s.samples[s.it].Timestamp), float64(s.samples[s.it].Value)
}

func (s samplesIterator) Err() error {
	return nil
}

func (s samplesIterator) AtFloatHistogram(*histogram.FloatHistogram) (int64, *histogram.FloatHistogram) {
	return 0, nil
}

func (s samplesIterator) AtHistogram(*histogram.Histogram) (int64, *histogram.Histogram) {
	return 0, nil
}

func (s samplesIterator) AtT() int64 {
	return int64(s.samples[s.it].Timestamp)
}

func (s samplesIterator) AtST() int64 {
	return 0
}

type storageSeries struct {
	l labels.Labels
	s []model.SamplePair
}

func newSeries(l labels.Labels, samples []model.SamplePair) storageSeries {
	return storageSeries{l, samples}
}

func (s storageSeries) Labels() labels.Labels {
	return s.l
}

func (s storageSeries) Iterator(_ chunkenc.Iterator) chunkenc.Iterator {
	return newSamplesIterator(s.s)
}
