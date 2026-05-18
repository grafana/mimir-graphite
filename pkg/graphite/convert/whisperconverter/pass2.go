package whisperconverter

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log/level"
	"github.com/pkg/errors"

	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/prometheus/prometheus/model/labels"
	promtsdb "github.com/prometheus/prometheus/tsdb"

	"github.com/grafana/mimir-graphite/v2/pkg/graphite/convert"
	"github.com/grafana/mimir-graphite/v2/pkg/tsdb"
)

// CommandPass2 performs the second pass conversion to Mimir blocks. It reads
// each intermediate file, sorts the metrics by labels, and outputs the block.
func (c *WhisperConverter) CommandPass2(intermediateDir, blocksDir string, overwriteBlocks bool) error {
	err := os.MkdirAll(filepath.Join(blocksDir, "wal"), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "could not create blocks directory")
	}

	fileChan := make(chan string)

	wgReads := &sync.WaitGroup{}
	wgReads.Add(c.threads)
	for i := 0; i < c.threads; i++ {
		go c.createBlocksFromChan(blocksDir, fileChan, wgReads)
	}
	c.getIntermediateListIntoChan(intermediateDir, blocksDir, overwriteBlocks, fileChan)

	wgReads.Wait()

	return nil
}

type metricsIndexEntry struct {
	Labels labels.Labels
	Pos    int64
}

// buildMetricsIndex converts the raw name->position index to a sorted
// labels->position index.
func buildMetricsIndex(nameIndex map[string]int64) []metricsIndexEntry {
	// We need to sort by label sort, so rebuild the labels and sort.
	index := make([]metricsIndexEntry, len(nameIndex))
	idx := 0
	for name, pos := range nameIndex {
		labelsBuilder := labels.NewBuilder(nil)
		index[idx] = metricsIndexEntry{
			Labels: convert.LabelsFromUntaggedName(name, labelsBuilder),
			Pos:    pos,
		}
		idx++
	}

	sort.Slice(index, func(i, j int) bool {
		return labels.Compare(index[i].Labels, index[j].Labels) == -1
	})

	return index
}

// createBlocksFromChan reads filenames from a channel and converts them to
// Mimir blocks.
func (c *WhisperConverter) createBlocksFromChan(blocksDir string, files chan string, wg *sync.WaitGroup) {
	for fname := range files {
		err := c.createOneBlock(fname, blocksDir)
		if err != nil {
			level.Error(c.logger).Log("msg", "Error creating block", "file", fname, "err", err)
			os.Exit(1)
		}
		c.progress.IncProcessed()
	}
	wg.Done()
}

// createOneBlock converts one intermediate file to a Mimir block.
func (c *WhisperConverter) createOneBlock(fname, blocksDir string) error {
	level.Info(c.logger).Log("file", fname, "msg", "creating block from intermediate file")
	i, err := convert.NewUSTableForRead(fname, convert.NewMimirSeriesProto, c.logger)
	if err != nil {
		return err
	}
	defer func() {
		_ = i.Close()
	}()
	index, err := i.Index()
	if err != nil {
		return err
	}

	if len(index) == 0 {
		level.Warn(c.logger).Log("msg", "no series for intermediate file", "file", fname)
		return nil
	}

	builder, err := tsdb.NewBuilder(blocksDir, tsdb.DefaultOptions())
	if err != nil {
		return err
	}

	metricsIndex := buildMetricsIndex(index)
	for _, info := range metricsIndex {
		var value convert.ProtoUnmarshaler
		_, value, err = i.ReadAt(info.Pos)
		if err != nil {
			return err
		}

		ms, ok := value.(*mimirpb.TimeSeries)
		if !ok {
			return convert.ErrBadData
		}

		labels := mimirpb.FromLabelAdaptersToLabels(ms.Labels)
		if len(c.customLabels) != 0 {
			labels = append(labels, c.customLabels...)
			sort.Sort(labels)
		}

		s := convert.NewMimirSeries(labels, ms.Samples)
		err = builder.AddSeriesWithSamples(s.Labels(), s.Iterator(nil))
		if err != nil {
			return err
		}
	}
	_, err = builder.FinishBlock(context.Background(), func(meta promtsdb.BlockMeta) interface{} { return meta })
	return err
}

// getIntermediateListIntoChan feeds intermediate files that need to be
// converted to blocks into the channel. If resume is enabled, first it builds a
// list of blocks that have already been generated. Then it walks the
// intermediate file directory looking for the requested dates and puts the
// files that are not already processed into the given channel. Intermediate
// files that have no data generate no block output, so they will always be
// reprocesssed when pass2 runs.
func (c *WhisperConverter) getIntermediateListIntoChan(intermediateDir, blocksDir string, overwriteBlocks bool, fileChan chan string) {
	skippableDates := make(map[time.Time]bool)
	if !overwriteBlocks {
		var err error
		skippableDates, err = convert.GetFinishedBlockDates(blocksDir)
		if err != nil {
			level.Warn(c.logger).Log("msg", "error iterating over blocks directory, unable to resume", "err", err)
		}
	}

	var paths []string
	for _, d := range c.dates {
		if _, ok := skippableDates[d]; ok {
			level.Info(c.logger).Log("date", d, "msg", "block already completely processed in previous run, skipping")
			c.progress.IncSkipped()
			continue
		}
		fname := filepath.Join(intermediateDir, d.Format("2006-01-02.intermediate"))
		_, err := os.Stat(fname)
		if err != nil {
			level.Error(c.logger).Log("file", fname, "msg", "did not find expected file", "err", err)
			// TODO: properly handle this error better rather than exiting.
			os.Exit(1)
		}
		paths = append(paths, fname)
	}
	paths = convert.PathsForWorker(paths, c.workerCount, c.workerID)

	for _, path := range paths {
		fileChan <- path
	}

	close(fileChan)
}
