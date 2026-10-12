package convert

import (
	"testing"

	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/stretchr/testify/require"
)

func TestMimirSeriesIterator_BasicIteration(t *testing.T) {
	s := MimirSeries{
		l: labels.EmptyLabels(),
		s: []mimirpb.Sample{
			{TimestampMs: 1000, Value: 10},
			{TimestampMs: 1010, Value: 20},
			{TimestampMs: 1020, Value: 100},
			{TimestampMs: 1030, Value: 15},
			{TimestampMs: 1050, Value: 42},
		},
	}

	it := s.Iterator(nil)

	require.Equal(t, chunkenc.ValFloat, it.Next())
	requireSampleEquals(t, it, 1000, 10)
	require.Equal(t, chunkenc.ValFloat, it.Next())
	requireSampleEquals(t, it, 1010, 20)
	require.Equal(t, chunkenc.ValFloat, it.Next())
	requireSampleEquals(t, it, 1020, 100)
	require.Equal(t, chunkenc.ValFloat, it.Next())
	requireSampleEquals(t, it, 1030, 15)
	require.Equal(t, chunkenc.ValFloat, it.Next())
	requireSampleEquals(t, it, 1050, 42)
	require.Equal(t, chunkenc.ValNone, it.Next())
}

func TestMimirSeriesIterator_Seek(t *testing.T) {
	s := MimirSeries{
		l: labels.EmptyLabels(),
		s: []mimirpb.Sample{
			{TimestampMs: 1000, Value: 10},
			{TimestampMs: 1010, Value: 20},
			{TimestampMs: 1020, Value: 100},
			{TimestampMs: 1030, Value: 15},
			{TimestampMs: 1050, Value: 42},
		},
	}

	it := s.Iterator(nil)
	require.Equal(t, chunkenc.ValFloat, it.Next())
	require.Equal(t, chunkenc.ValFloat, it.Next())
	require.Equal(t, chunkenc.ValFloat, it.Next())
	require.Equal(t, chunkenc.ValFloat, it.Next())
	// Seeking has no effect if the current timestamp is already past the
	// requested time.
	require.Equal(t, chunkenc.ValFloat, it.Seek(1000))
	requireSampleEquals(t, it, 1030, 15)

	require.Equal(t, chunkenc.ValFloat, it.Seek(1050))
	requireSampleEquals(t, it, 1050, 42)

	// We have to reset the iterator to go backwards
	it = s.Iterator(nil)
	require.Equal(t, chunkenc.ValNone, it.Seek(50000))
	require.NotNil(t, it.Err())

	it = s.Iterator(nil)
	require.Equal(t, chunkenc.ValFloat, it.Seek(1015))
	requireSampleEquals(t, it, 1020, 100)
}

func requireSampleEquals(t *testing.T, it chunkenc.Iterator, ts int64, v float64) {
	require.Nil(t, it.Err())
	gotTS, gotV := it.At()
	require.Equal(t, ts, gotTS)
	require.Equal(t, v, gotV)
}
