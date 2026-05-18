package whisperconverter

import (
	"math/rand"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/mimir-graphite/v2/pkg/graphite/convert"
)

// TestCommandPass2 is a sanity-check for pass2.  It confirms that data is
// generated but does not check that it is valid.
func TestCommandPass2(t *testing.T) {
	tmpIntermediateDir, err := os.MkdirTemp("/tmp", "TestCommandPass2.intermediate-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpIntermediateDir)
	}()

	tmpBlockDir, err := os.MkdirTemp("/tmp", "TestCommandPass2.blocks-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpBlockDir)
	}()

	data1 := createData([]string{"foo.bar.baz", "my.cool.metric", "something.else"})
	data2 := createData([]string{"my.cool.metric", "something.else", "unique.metric"})
	data3 := createData([]string{"my.cool.metric", "something.else", "what.does.the.fox.say"})

	err = createIntermediate(tmpIntermediateDir+"/2022-08-01.intermediate", data1)
	require.NoError(t, err)
	err = createIntermediate(tmpIntermediateDir+"/2022-08-02.intermediate", data2)
	require.NoError(t, err)
	err = createIntermediate(tmpIntermediateDir+"/2022-08-03.intermediate", data3)
	require.NoError(t, err)

	dates := make([]time.Time, 0)
	startDate, err := ToTime("2022-08-01")
	require.NoError(t, err)
	endDate, err := ToTime("2022-08-03")
	require.NoError(t, err)
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}

	c := NewWhisperConverter(
		"",
		"",
		regexp.MustCompile(`\.wsp$`),
		2,
		1,
		0,
		labels.FromStrings("customlabel", "val"),
		dates,
		log.NewNopLogger(),
	)

	err = c.CommandPass2(tmpIntermediateDir, tmpBlockDir, true)
	require.NoError(t, err)

	blockToRemove := checkBlockSimpleValid(t, tmpBlockDir)

	// Test resume by removing one of the blocks
	_ = os.RemoveAll(tmpBlockDir + "/" + blockToRemove)

	// Rerun the test, keeping blocks that still exist.
	c = NewWhisperConverter(
		"",
		"",
		regexp.MustCompile(`\.wsp$`),
		2,
		1,
		0,
		labels.FromStrings("customlabel", "val"),
		dates,
		log.NewNopLogger(),
	)

	err = c.CommandPass2(tmpIntermediateDir, tmpBlockDir, false)
	require.NoError(t, err)

	checkBlockSimpleValid(t, tmpBlockDir)
}

// createData returns some fake data, using the passed-in metricNames (which
// should be in dotted format)
func createData(metricNames []string) map[string]*mimirpb.TimeSeries {
	numSamples := 1000

	data := make(map[string]*mimirpb.TimeSeries)
	for _, m := range metricNames {
		labelsBuilder := labels.NewBuilder(nil)
		labels := convert.LabelsFromUntaggedName(m, labelsBuilder)
		samples := make([]mimirpb.Sample, numSamples)
		for i := 0; i < numSamples; i++ {
			samples[i] = mimirpb.Sample{
				TimestampMs: int64((i + 1000000) * 1000),
				Value:       rand.NormFloat64(),
			}
		}
		data[m] = &mimirpb.TimeSeries{
			Labels:  mimirpb.FromLabelsToLabelAdapters(labels),
			Samples: samples,
		}
	}

	return data
}

// createIntermediate generates an intermediate file in the given path,
// writing the given time series to the file.
func createIntermediate(path string, data map[string]*mimirpb.TimeSeries) error {
	table, err := convert.NewUSTableForAppend(path, true, convert.NewMimirSeriesProto, log.NewNopLogger())
	if err != nil {
		return err
	}
	defer func() {
		_ = table.Close()
	}()

	for k, v := range data {
		err = table.Append(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkBlockSimpleValid does a basic check to see if the output is valid.
// returns one example representative block directory.
func checkBlockSimpleValid(t *testing.T, path string) string {
	actualBlocks, err := ListFilesInDir(path)
	require.NoError(t, err)

	foundWal := false
	exampleBlock := ""
	for _, d := range actualBlocks {
		if d == "wal" {
			foundWal = true
			continue
		}
		if exampleBlock == "" {
			exampleBlock = d
		}
		actualFiles, err := ListFilesInDir(path + "/" + d)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"chunks", "index", "meta.json"}, actualFiles)
	}
	require.True(t, foundWal)

	return exampleBlock
}
