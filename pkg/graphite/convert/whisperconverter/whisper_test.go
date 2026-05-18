package whisperconverter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/mimir/pkg/mimirpb"
	log2 "github.com/grafana/mimir/pkg/util/log"
	"github.com/kisielk/whisper-go/whisper"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/stretchr/testify/require"

	"github.com/grafana/mimir-graphite/v2/pkg/graphite/convert"
)

func simpleArchiveInfo(points, secondsPerPoint int) whisper.ArchiveInfo {
	return whisper.ArchiveInfo{
		Offset:          0,
		SecondsPerPoint: uint32(secondsPerPoint),
		Points:          uint32(points),
	}
}

func TestExtractWhisperPoints(t *testing.T) {
	tests := []struct {
		name       string
		metricName string
		archive    *testArchive
		want       []whisper.Point
		wantErr    bool
	}{
		{
			name:       "empty archive",
			metricName: "mymetric",
			archive:    &testArchive{},
			wantErr:    true,
		},
		{
			name:       "simple series, remove zeroes",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					simpleArchiveInfo(4, 1),
				},
				points: [][]whisper.Point{
					{
						whisper.NewPoint(time.Unix(0, 0), 1),
						whisper.NewPoint(time.Unix(1000, 0), 1),
						whisper.NewPoint(time.Unix(1001, 0), 2),
						whisper.NewPoint(time.Unix(1002, 0), 4),
					},
				},
			},
			want: []whisper.Point{
				{
					Timestamp: 1000,
					Value:     1,
				},
				{
					Timestamp: 1001,
					Value:     2,
				},
				{
					Timestamp: 1002,
					Value:     4,
				},
			},
		},
		{
			name:       "simple series, large retention",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					simpleArchiveInfo(6000, 1),
					simpleArchiveInfo(6000, 60),
				},
				points: [][]whisper.Point{
					{
						whisper.NewPoint(time.Unix(1000, 0), 1),
					},
					// None of the points in this archive are valid because all points
					// are covered by the first archive.
					{
						whisper.NewPoint(time.Unix(940, 0), 2),
					},
				},
			},
			want: []whisper.Point{
				{
					Timestamp: 1000,
					Value:     1,
				},
			},
		},
		{
			name:       "multiple series, different intervals, zeros, duplicate points",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					simpleArchiveInfo(120, 1),
					simpleArchiveInfo(4, 60),
				},
				points: [][]whisper.Point{
					{
						whisper.NewPoint(time.Unix(0, 0), 12),   // skipped
						whisper.NewPoint(time.Unix(900, 0), 12), // skipped due to being out of retention
						whisper.NewPoint(time.Unix(1054, 0), 12),
						whisper.NewPoint(time.Unix(1055, 0), 42),
						whisper.NewPoint(time.Unix(1060, 0), 2),
						whisper.NewPoint(time.Unix(1056, 0), 27.5),
					},
					{
						whisper.NewPoint(time.Unix(0, 0), 12),     // skipped
						whisper.NewPoint(time.Unix(1058, 0), 1),   // skipped, covered by other archive
						whisper.NewPoint(time.Unix(1060, 0), 102), // duplicate, the one in the archive above should be kept and this one skipped
						whisper.NewPoint(time.Unix(650, 0), 50),   // skipped due to being out of retention
						whisper.NewPoint(time.Unix(1055, 0), 5),   // skipped, covered by other archive
						whisper.NewPoint(time.Unix(901, 0), 4),
					},
				},
			},
			want: []whisper.Point{
				// We do not to any rounding / conversion of time values.
				{
					Timestamp: 901,
					Value:     4,
				},
				{
					Timestamp: 1054,
					Value:     12,
				},
				{
					Timestamp: 1055,
					Value:     42,
				},
				{
					Timestamp: 1056,
					Value:     27.5,
				},
				{
					Timestamp: 1060,
					Value:     2,
				},
			},
		},
		{
			name:       "single series, multiple archives and retentions, with duplicates and points beyond retention",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					// This is what the test will define
					// Maxts                        Mints
					// 1030     1020     1009     994
					//  [         ]
					//  [                  ]
					//  [                           ]
					// And this is what the test will expect
					// 1030     1020     1009     994
					//  [XXXXXXXXX]
					//  [          XXXXXXXXX]
					//  [                   XXXXXXXX]

					simpleArchiveInfo(10, 1),
					simpleArchiveInfo(7, 3),
					simpleArchiveInfo(6, 6),
				},
				points: [][]whisper.Point{
					{
						whisper.NewPoint(time.Unix(1020, 0), 20), // Skipped, this is past the lower bound of this archive.
						whisper.NewPoint(time.Unix(1021, 0), 21),
						whisper.NewPoint(time.Unix(1022, 0), 22),
						whisper.NewPoint(time.Unix(1023, 0), 23),
						whisper.NewPoint(time.Unix(1024, 0), 24),
						whisper.NewPoint(time.Unix(1025, 0), 25),
						whisper.NewPoint(time.Unix(1026, 0), 26),
						whisper.NewPoint(time.Unix(1027, 0), 27),
						whisper.NewPoint(time.Unix(1028, 0), 28),
						whisper.NewPoint(time.Unix(1029, 0), 29),
						whisper.NewPoint(time.Unix(1030, 0), 30),
					},
					{
						whisper.NewPoint(time.Unix(1009, 0), 9), // Skipped, this is past the lower bound of this archive.
						whisper.NewPoint(time.Unix(1012, 0), 12),
						whisper.NewPoint(time.Unix(1015, 0), 15),
						whisper.NewPoint(time.Unix(1018, 0), 18),
						whisper.NewPoint(time.Unix(1021, 0), 21), // skipped
						whisper.NewPoint(time.Unix(1024, 0), 24), // skipped
						whisper.NewPoint(time.Unix(1027, 0), 27), // skipped
						whisper.NewPoint(time.Unix(1030, 0), 30), // skipped
					},
					{
						whisper.NewPoint(time.Unix(1000, 0), 0),
						whisper.NewPoint(time.Unix(1006, 0), 6),
						whisper.NewPoint(time.Unix(1009, 0), 99), // This is the upper bound of this archive
						whisper.NewPoint(time.Unix(1012, 0), 12), // skipped
						whisper.NewPoint(time.Unix(1018, 0), 18), // skipped
						whisper.NewPoint(time.Unix(1024, 0), 24), // skipped
						whisper.NewPoint(time.Unix(1030, 0), 30), // skipped
					},
				},
			},
			want: []whisper.Point{
				{
					Timestamp: 1000,
					Value:     0,
				},
				{
					Timestamp: 1006,
					Value:     6,
				},
				{
					Timestamp: 1009,
					Value:     99,
				},
				{
					Timestamp: 1012,
					Value:     12,
				},
				{
					Timestamp: 1015,
					Value:     15,
				},
				{
					Timestamp: 1018,
					Value:     18,
				},
				{
					Timestamp: 1021,
					Value:     21,
				},
				{
					Timestamp: 1022,
					Value:     22,
				},
				{
					Timestamp: 1023,
					Value:     23,
				},
				{
					Timestamp: 1024,
					Value:     24,
				},
				{
					Timestamp: 1025,
					Value:     25,
				},
				{
					Timestamp: 1026,
					Value:     26,
				},
				{
					Timestamp: 1027,
					Value:     27,
				},
				{
					Timestamp: 1028,
					Value:     28,
				},
				{
					Timestamp: 1029,
					Value:     29,
				},
				{
					Timestamp: 1030,
					Value:     30,
				},
			},
		},
		{
			name:       "test retention when archives are empty",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					simpleArchiveInfo(10, 1),
					simpleArchiveInfo(7, 3),
					simpleArchiveInfo(6, 6),
				},
				points: [][]whisper.Point{
					{},
					{},
					{},
				},
			},
			want: []whisper.Point{},
		},
		{
			name:       "test retention when first archives are empty",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					simpleArchiveInfo(10, 1),
					simpleArchiveInfo(7, 3),
					simpleArchiveInfo(6, 6),
				},
				points: [][]whisper.Point{
					{},
					{
						whisper.NewPoint(time.Unix(1009, 0), 9),
					},
					{
						whisper.NewPoint(time.Unix(1009, 0), 99), // skipped because archive 1 has a point at this time
						whisper.NewPoint(time.Unix(998, 0), 12),
					},
				},
			},
			want: []whisper.Point{
				{
					Timestamp: 998,
					Value:     12,
				},
				{
					Timestamp: 1009,
					Value:     9,
				},
			},
		},
		{
			name:       "test retention when first archives are empty",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					simpleArchiveInfo(10, 1),
					simpleArchiveInfo(7, 3),
					simpleArchiveInfo(6, 6),
				},
				points: [][]whisper.Point{
					{},
					{},
					{
						whisper.NewPoint(time.Unix(1009, 0), 99), // skipped because archive 1 has a point at this time
						whisper.NewPoint(time.Unix(1000, 0), 12),
					},
				},
			},
			want: []whisper.Point{
				{
					Timestamp: 1000,
					Value:     12,
				},
				{
					Timestamp: 1009,
					Value:     99,
				},
			},
		},
		{
			name:       "simple series, ordering is fixed",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					simpleArchiveInfo(3, 1),
				},
				points: [][]whisper.Point{
					{
						whisper.NewPoint(time.Unix(2002, 0), 4),
						whisper.NewPoint(time.Unix(2000, 0), 87),
						whisper.NewPoint(time.Unix(2001, 0), 112),
					},
				},
			},
			want: []whisper.Point{
				{
					Timestamp: 2000,
					Value:     87,
				},
				{
					Timestamp: 2001,
					Value:     112,
				},
				{
					Timestamp: 2002,
					Value:     4,
				},
			},
		},
		{
			name:       "bad archive",
			metricName: "mymetric",
			archive: &testArchive{
				infos: []whisper.ArchiveInfo{
					simpleArchiveInfo(3, 1),
				},
				points: [][]whisper.Point{
					{
						whisper.NewPoint(time.Unix(1000, 0), 1),
						whisper.NewPoint(time.Unix(1001, 0), 2),
						whisper.NewPoint(time.Unix(1002, 0), 4),
					},
				},
				err: fmt.Errorf("something is wrong with this archive"),
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ReadPoints(test.archive, test.metricName)
			if !test.wantErr {
				require.NoError(t, err)
				require.EqualValues(t, test.want, got)
			} else {
				require.NotNil(t, err)
				require.Nil(t, test.want)
			}
		})
	}
}

func TestConvertToMimirSamples(t *testing.T) {
	tests := []struct {
		name        string
		metricName  string
		points      []whisper.Point
		wantLabels  labels.Labels
		wantSamples []mimirpb.Sample
		wantErr     bool
	}{
		{
			name:        "no points",
			metricName:  "mymetric",
			points:      []whisper.Point{},
			wantLabels:  nil,
			wantSamples: nil,
			wantErr:     true,
		},
		{
			name:       "simple series",
			metricName: "something.somewhere.hosts.cluster-prod-app-03_cool-dub_example_com.cpu_usage",
			points: []whisper.Point{
				whisper.NewPoint(time.Unix(1000, 0), 12),
				whisper.NewPoint(time.Unix(1001, 0), 42),
				whisper.NewPoint(time.Unix(1004, 0), 27.5),
			},
			wantLabels: labels.Labels{
				{
					Name:  "__n000__",
					Value: "something",
				},
				{
					Name:  "__n001__",
					Value: "somewhere",
				},
				{
					Name:  "__n002__",
					Value: "hosts",
				},
				{
					Name:  "__n003__",
					Value: "cluster-prod-app-03_cool-dub_example_com",
				},
				{
					Name:  "__n004__",
					Value: "cpu_usage",
				},
				{
					Name:  "__name__",
					Value: "graphite_untagged",
				},
			},
			wantSamples: []mimirpb.Sample{
				{
					TimestampMs: 1000000,
					Value:       12,
				},
				{
					TimestampMs: 1001000,
					Value:       42,
				},
				{
					TimestampMs: 1004000,
					Value:       27.5,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSamples, gotErr := ToMimirSamples(test.points)
			// This is sort of testing someone else's library, but it is useful here
			// to show how the conversion works.
			labelsBuilder := labels.NewBuilder(nil)
			gotLabels := convert.LabelsFromUntaggedName(test.metricName, labelsBuilder)
			if test.wantErr {
				require.Nil(t, gotSamples)
				require.NotNil(t, gotErr)
			} else {
				require.Equal(t, test.wantLabels, gotLabels)
				require.Equal(t, test.wantSamples, gotSamples)
				require.Nil(t, gotErr)
			}
		})
	}
}

func utcSample(year int, month time.Month, day, hour, minute, second int, v float64) mimirpb.Sample {
	return mimirpb.Sample{
		TimestampMs: time.Date(year, month, day, hour, minute, second, 0, time.UTC).UnixMilli(),
		Value:       v,
	}
}

func TestConvertSampleSplitter(t *testing.T) {
	tests := []struct {
		name        string
		samples     []mimirpb.Sample
		wantSamples [][]mimirpb.Sample
	}{
		{
			name:        "no points",
			samples:     []mimirpb.Sample{},
			wantSamples: [][]mimirpb.Sample{},
		},
		{
			name: "start exactly at midnight",
			samples: []mimirpb.Sample{
				utcSample(2022, 4, 20, 0, 0, 0, 10.2),
				utcSample(2022, 4, 20, 0, 0, 1, 5),
				utcSample(2022, 4, 20, 0, 2, 1, 68),
				utcSample(2022, 4, 20, 5, 0, 1, 42),
				utcSample(2022, 4, 20, 11, 59, 1, 100),
			},
			wantSamples: [][]mimirpb.Sample{
				{
					utcSample(2022, 4, 20, 0, 0, 0, 10.2),
					utcSample(2022, 4, 20, 0, 0, 1, 5),
					utcSample(2022, 4, 20, 0, 2, 1, 68),
					utcSample(2022, 4, 20, 5, 0, 1, 42),
					utcSample(2022, 4, 20, 11, 59, 1, 100),
				},
			},
		},
		{
			name: "ending at midnight, new chunk",
			samples: []mimirpb.Sample{
				utcSample(2022, 4, 19, 11, 59, 59, 10.2),
				utcSample(2022, 4, 20, 0, 0, 0, 10.2),
				utcSample(2022, 4, 20, 0, 0, 1, 56),
				utcSample(2022, 4, 20, 0, 2, 1, 29),
				utcSample(2022, 4, 20, 5, 0, 1, 37),
				utcSample(2022, 4, 21, 0, 0, 0, 2),
			},
			wantSamples: [][]mimirpb.Sample{
				{
					utcSample(2022, 4, 19, 11, 59, 59, 10.2),
				},
				{
					utcSample(2022, 4, 20, 0, 0, 0, 10.2),
					utcSample(2022, 4, 20, 0, 0, 1, 56),
					utcSample(2022, 4, 20, 0, 2, 1, 29),
					utcSample(2022, 4, 20, 5, 0, 1, 37),
				},
				{
					utcSample(2022, 4, 21, 0, 0, 0, 2),
				},
			},
		},
		{
			name: "timestamps very far apart",
			samples: []mimirpb.Sample{
				utcSample(2022, 4, 20, 0, 0, 0, 10.2),
				utcSample(2022, 5, 20, 0, 0, 1, 5),
				utcSample(2023, 5, 20, 0, 0, 1, 68),
			},
			wantSamples: [][]mimirpb.Sample{
				{
					utcSample(2022, 4, 20, 0, 0, 0, 10.2),
				},
				{
					utcSample(2022, 5, 20, 0, 0, 1, 5),
				},
				{
					utcSample(2023, 5, 20, 0, 0, 1, 68),
				},
			},
		},
		{
			name: "another silly case",
			samples: []mimirpb.Sample{
				utcSample(2022, 4, 20, 0, 0, 0, 10.2),
				utcSample(2022, 5, 20, 0, 0, 1, 5),
				utcSample(2022, 5, 20, 3, 0, 1, 68),
			},
			wantSamples: [][]mimirpb.Sample{
				{
					utcSample(2022, 4, 20, 0, 0, 0, 10.2),
				},
				{
					utcSample(2022, 5, 20, 0, 0, 1, 5),
					utcSample(2022, 5, 20, 3, 0, 1, 68),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSamples := SplitSamplesByDays(test.samples)
			require.Equal(t, test.wantSamples, gotSamples)
		})
	}
}

func TestConvertToMimirBlocks(t *testing.T) {
	testFilePath := "./testdata/test.wsp"
	blockDir, err := os.MkdirTemp("/tmp", "mimirblocktest*")
	require.Nil(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(blockDir)
	})

	err = os.Mkdir(filepath.Join(blockDir, "wal"), 0700)
	require.Nil(t, err)

	metricName := "foo.bar.baz.test"

	samples, err := WhisperToMimirSamples(testFilePath, metricName)
	require.Nil(t, err)

	labelsBuilder := labels.NewBuilder(nil)
	labels := convert.LabelsFromUntaggedName(metricName, labelsBuilder)

	series := []storage.Series{convert.NewMimirSeries(labels, samples)}
	blockFName, err := tsdb.CreateBlock(
		series,
		blockDir,
		30*time.Second.Milliseconds(),
		log2.SlogFromGoKit(log.NewNopLogger()),
	)
	require.Nil(t, err)

	goldenOutputFilePath := "./testdata/golden.block"
	wantFile, err := os.ReadFile(goldenOutputFilePath)
	require.Nil(t, err)
	gotFile, err := os.ReadFile(filepath.Join(blockFName, "chunks", "000001"))
	require.Nil(t, err)
	require.Equal(t, wantFile, gotFile)

	goldenMetaFilePath := "./testdata/golden.meta.json"
	wantFile, err = os.ReadFile(goldenMetaFilePath)
	require.Nil(t, err)
	wantBlock := tsdb.BlockMeta{}
	err = json.Unmarshal(wantFile, &wantBlock)
	require.Nil(t, err)
	gotFile, err = os.ReadFile(filepath.Join(blockFName, "meta.json"))
	require.Nil(t, err)
	gotBlock := tsdb.BlockMeta{}
	err = json.Unmarshal(gotFile, &gotBlock)
	require.Nil(t, err)

	// The UUID changes every time, so forcibly make them equal before comparing.
	wantBlock.ULID = gotBlock.ULID
	wantBlock.Compaction.Sources[0] = gotBlock.Compaction.Sources[0]
	require.Equal(t, wantBlock, gotBlock)
}

type testArchive struct {
	infos  []whisper.ArchiveInfo
	points [][]whisper.Point
	err    error
}

func (a *testArchive) GetArchives() []whisper.ArchiveInfo {
	return a.infos
}

func (a *testArchive) DumpArchive(n int) ([]whisper.Point, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.points[n], nil
}
