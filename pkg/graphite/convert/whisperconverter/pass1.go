package whisperconverter

import (
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kit/log/level"
	"github.com/pkg/errors"

	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/mimir-graphite/v2/pkg/graphite/convert"
)

// CommandPass1 performs the first pass conversion to intermediate files. Each
// metric is dumped, sorted, and split into days, and the individual days are
// written to the intermediate files.  If this stage crashes, rerunning the
// stage will automatically resume. targetWhisperFiles is a filename containing
// the list of files to process, or if blank, files will be walked using
// c.whisperDirectory.
func (c *WhisperConverter) CommandPass1(targetWhisperFiles, intermediateDir string, resumeIntermediate bool) error {
	err := os.MkdirAll(intermediateDir, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "could not create intermediate directory")
	}

	fileChan := make(chan string)

	wgReads := &sync.WaitGroup{}
	wgReads.Add(c.threads)

	intermediateFiles := c.openIntermediateFiles(intermediateDir, resumeIntermediate)
	defer func() {
		for _, i := range intermediateFiles {
			_ = i.Close()
		}
	}()

	progressFName := filepath.Join(intermediateDir, "processedMetrics.intermediate")
	progressFile, skippableMetrics, err := convert.NewUSTableForAppendWithIndex(progressFName, false, convert.NewMimirSeriesProto, c.logger)
	if err != nil {
		return errors.Wrap(err, "error opening processsedMetrics intermediate file")
	}
	defer func() {
		_ = progressFile.Close()
	}()

	for i := 0; i < c.threads; i++ {
		go c.createIntermediateFromChan(fileChan, intermediateFiles, progressFile, skippableMetrics, wgReads)
	}
	c.getWhisperListIntoChan(targetWhisperFiles, fileChan)

	wgReads.Wait()
	return nil
}

// openIntermediateFiles creates or opens intermediate files for appending as
// part of pass one. It returns the opened files.
func (c *WhisperConverter) openIntermediateFiles(intermediateDir string, resumeIntermediate bool) (intermediateFiles map[time.Time]*convert.USTable) {
	intermediateFiles = make(map[time.Time]*convert.USTable)

	dateChan := make(chan time.Time)
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	wg.Add(c.threads)
	for i := 0; i < c.threads; i++ {
		go func() {
			for d := range dateChan {
				level.Info(c.logger).Log("msg", "Opening intermediate file for date", "date", d)
				fname := filepath.Join(intermediateDir, d.Format("2006-01-02.intermediate"))
				intermediate, err := convert.NewUSTableForAppend(fname, !resumeIntermediate, convert.NewMimirSeriesProto, c.logger)
				if err != nil {
					level.Error(c.logger).Log("msg", "error opening intermediate file", "err", err)
					// TODO: properly handle this error better rather than exiting.
					os.Exit(1)
				}

				mu.Lock()
				intermediateFiles[d] = intermediate
				mu.Unlock()
			}
			wg.Done()
		}()
	}

	for _, d := range c.dates {
		dateChan <- d
	}
	close(dateChan)
	wg.Wait()

	return intermediateFiles
}

// createIntermediateFromChan reads whisper archives, converts the data to
// Mimir points, splits the data by UTC date, and then sends the blocks to the
// receiving channel corresponding to each date. If there is no channel for a
// given block, it is silently skipped.
func (c *WhisperConverter) createIntermediateFromChan(files chan string, intermediateFiles map[time.Time]*convert.USTable, progressFile *convert.USTable, skippableMetrics map[string]int64, wg *sync.WaitGroup) {
	for fname := range files {
		metricName := c.getMetricName(fname)
		if _, ok := skippableMetrics[metricName]; ok {
			level.Info(c.logger).Log("file", fname, "metric", metricName, "msg", "already completely processed in previous run, skipping")
			c.progress.IncSkipped()
			continue
		}
		level.Info(c.logger).Log("file", fname, "metric", metricName, "msg", "processing file")

		samples, err := WhisperToMimirSamples(fname, metricName)
		if err != nil {
			level.Warn(c.logger).Log("file", fname, "metric", metricName, "msg", "error converting whisper metric", "err", err)
			c.progress.IncSkipped()
			continue
		}
		labelsBuilder := labels.NewBuilder(labels.EmptyLabels())
		labels := convert.LabelsFromUntaggedName(metricName, labelsBuilder)

		blocks := SplitSamplesByDays(samples)
		// Shuffle blocks so we write dates to channels in random order, reducing
		// contention.
		rand.Shuffle(len(blocks), func(i, j int) { blocks[i], blocks[j] = blocks[j], blocks[i] })

		wroteDates := make(map[time.Time]bool)
		for _, block := range blocks {
			t := time.UnixMilli(block[0].TimestampMs).UTC()
			// Can't use time.Truncate because some days do not have 24 hours (leap seconds, etc).
			rounded := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

			// It is not necessarily an error if we don't find a channel, we might
			// not be being asked to output for this date.
			if i, ok := intermediateFiles[rounded]; ok {
				level.Debug(c.logger).Log("msg", "writing data to intermediate file for date", "date", rounded)
				err = i.Append(metricName, &mimirpb.TimeSeries{
					Labels:  mimirpb.FromLabelsToLabelAdapters(labels),
					Samples: block,
				},
				)
				if err != nil {
					level.Error(c.logger).Log("metric", metricName, "msg", "error writing to intermediate file", "err", err)
					// TODO: properly handle this error better rather than exiting.
					os.Exit(1)
				}
				wroteDates[rounded] = true
			}
		}

		// Write to the progress file to indicate this one is done.
		err = progressFile.Append(metricName, &mimirpb.TimeSeries{})
		if err != nil {
			level.Error(c.logger).Log("metric", metricName, "msg", "error writing to processsedMetrics intermediate file", "err", err)
			// TODO: properly handle this error better rather than exiting.
			os.Exit(1)
		}

		if len(wroteDates) == 0 {
			level.Warn(c.logger).Log("metric", metricName, "msg", "didn't write any output (file may not cover selected dates)")
		}

		c.progress.IncProcessed()
	}
	wg.Done()
}
