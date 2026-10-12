package tsdb

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/grafana/dskit/multierror"
	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/index"
)

const (
	metaVersion1 = 1

	// This constant is used by Prometheus, but it's not exported.
	samplesPerChunk = 120

	// Permissions for new files and directories. This is consistent with Prometheus code
	// (eg. index.NewWriter, chunks.NewWriter). User is expected to have umask set to avoid world-writeable
	// files and directories.
	permFile = 0o666
	permDir  = 0o777
)

// Builder helps to build TSDB block. All series that should be included in the TSDB block should be added using
// AddSeriesWithSamples method. After adding all series, FinishBlock will complete the writing of the block.
type Builder struct {
	opts Options

	blockID           ulid.ULID
	blockDir          string
	tempDir           string
	unsortedChunksDir string

	symbolsMtx sync.Mutex
	symbols    *symbolsBatcher

	seriesMtx sync.Mutex
	series    *seriesBatcher

	chunksForUnsortedSeriesMtx sync.Mutex
	chunksForUnsortedSeries    *chunks.Writer
}

// Options for Builder.
type Options struct {
	SymbolsBatchSize int // How many symbols to keep in memory, before flushing them to files.
	SeriesBatchSize  int // How many series to keep in memory, before flushing them to files.

	MinBlockTime time.Time // If not zero, samples with timestamp lower than this value will be ignored.
	MaxBlockTime time.Time // If not zero, samples with timestamp equal of higher than this value will be ignored.
}

// DefaultOptions returns default builder options that can be used in NewBuilder function.
func DefaultOptions() Options {
	return Options{
		SymbolsBatchSize: 10000,
		SeriesBatchSize:  10000,
	}
}

// NewBuilder creates builder for building a single TSDB block. Multiple builders can use the same work directory, each
// builder will create subdirectory for the block that it's building. This subdirectory will be a valid TSDB block
// only if at no point Builder returns an error from any of its methods.
func NewBuilder(workDirectory string, opts Options) (*Builder, error) {
	blockID := ulid.MustNew(ulid.Now(), rand.Reader)

	blockDir := filepath.Join(workDirectory, blockID.String())
	if err := os.MkdirAll(blockDir, permDir); err != nil {
		return nil, fmt.Errorf("failed to create block directory %v: %w", blockDir, err)
	}

	blockTempDir := filepath.Join(blockDir, "temp")
	if err := os.MkdirAll(blockTempDir, permDir); err != nil {
		return nil, fmt.Errorf("failed to create temp directory %v: %w", blockTempDir, err)
	}

	// Create unsorted chunks under temp dir, so that deleting temp dir will delete everything.
	unsortedChunksDir := filepath.Join(blockTempDir, "unsorted_chunks")
	if err := os.MkdirAll(unsortedChunksDir, permDir); err != nil {
		return nil, fmt.Errorf("failed to create chunks directory %v: %w", unsortedChunksDir, err)
	}

	cw, err := chunks.NewWriter(unsortedChunksDir)
	if err != nil {
		return nil, err
	}

	b := &Builder{
		opts:                    opts,
		blockID:                 blockID,
		blockDir:                blockDir,
		tempDir:                 blockTempDir,
		unsortedChunksDir:       unsortedChunksDir,
		symbols:                 newSymbolsBatcher(opts.SymbolsBatchSize, blockTempDir),
		series:                  newSeriesBatcher(opts.SymbolsBatchSize, blockTempDir),
		chunksForUnsortedSeries: cw,
	}

	// Add "" symbol. TSDB blocks produced by Prometheus always have it, even though it's not really used for anything.
	if err := b.symbols.addSymbol(""); err != nil {
		// If opts.SymbolsBatchSize is bigger than 1, this cannot fail.
		panic(err)
	}

	return b, nil
}

// AddSeriesWithSamples adds single series to the block builder. AddSeriesWithSamples can be called with series in
// random order, and even concurrently from different goroutines.
func (b *Builder) AddSeriesWithSamples(lbls labels.Labels, samples chunkenc.Iterator) error {
	minBlockTime, maxBlockTime := int64(0), int64(0)
	if !b.opts.MinBlockTime.IsZero() {
		minBlockTime = timestamp.FromTime(b.opts.MinBlockTime)
	}
	if !b.opts.MaxBlockTime.IsZero() {
		maxBlockTime = timestamp.FromTime(b.opts.MaxBlockTime)
	}

	chks, err := samplesToChunks(samples, minBlockTime, maxBlockTime)
	if err != nil {
		return fmt.Errorf("failed to convert samples to chunks: %w", err)
	}
	if len(chks) == 0 {
		// Samples produced no chunks (eg there were no samples, or all were outside of builder's min/max time range)
		return nil
	}

	if err := b.addSymbols(lbls); err != nil {
		return err
	}

	if err := b.writeChunksForUnsortedSeries(chks); err != nil {
		return err
	}

	return b.addSeries(lbls, chks)
}

// FinishBlock will build the final TSDB block from series added previously via AddSeriesWithSamples. FinishBlock should
// only be called once, after all calls to AddSeriesWithSamples have finished successfully. It is caller's responsibility
// to guarantee that, otherwise races will happen. Calling another AddSeriesWithSamples after FinishBlock has been
// called is undefined, and will likely panic for some reason.
//
// extendMeta function can return either passed meta, or return another object that will be stored into meta.json file.
// Eg. Grafana Mimir stores metadata.Meta (from Thanos) into the meta.json file.
func (b *Builder) FinishBlock(ctx context.Context, extendMeta func(tsdb.BlockMeta) interface{}) (ulid.ULID, error) {
	// We don't need any locking here, as caller guarantees that all calls to AddSeriesWithSamples have finished.
	merr := multierror.MultiError{}

	// Flush remaining symbols and series to files, and close chunks writer for unsorted chunks.
	merr.Add(b.symbols.flushSymbols(true))
	merr.Add(b.series.flushSeries(true))
	merr.Add(b.chunksForUnsortedSeries.Close())

	if err := merr.Err(); err != nil {
		return b.blockID, err
	}

	closers := []io.Closer(nil)
	defer func() {
		// If there is anything to close here, there must already be error returned by the outer function.
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	// Prepare writers and readers.
	indexWriter, err := index.NewWriter(ctx, filepath.Join(b.blockDir, "index"))
	if err != nil {
		return b.blockID, err
	}
	closers = append(closers, indexWriter)

	err = addSymbolsToIndexWriter(indexWriter, b.symbols.getSymbolFiles())
	if err != nil {
		return b.blockID, err
	}

	unsortedChunksReader, err := chunks.NewDirReader(b.unsortedChunksDir, nil)
	if err != nil {
		return b.blockID, err
	}
	closers = append(closers, unsortedChunksReader)

	chunksWriter, err := chunks.NewWriter(filepath.Join(b.blockDir, "chunks"))
	if err != nil {
		return b.blockID, err
	}
	closers = append(closers, chunksWriter)

	stats, minT, maxT, err := addSeriesToIndex(indexWriter, chunksWriter, b.series.getSeriesFiles(), unsortedChunksReader)
	if err != nil {
		return b.blockID, err
	}

	for len(closers) > 0 {
		c := closers[0]
		closers = closers[1:]

		if err1 := c.Close(); err1 != nil {
			return b.blockID, err1
		}
	}

	err = os.RemoveAll(b.tempDir)
	if err != nil {
		return b.blockID, fmt.Errorf("failed to delete temp files for the block: %w", err)
	}

	return b.blockID, writeMetaFile(b.blockID, b.blockDir, minT, maxT, stats, extendMeta)
}

// ReadMetaFile is a convenience function for reading meta files.
func ReadMetaFile(blockDir string) (*tsdb.BlockMeta, error) {
	metaPath := filepath.Join(blockDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	jsonMeta := &tsdb.BlockMeta{}
	err = json.Unmarshal(data, jsonMeta)
	return jsonMeta, err
}

// minT, maxT are min and max time of the samples in the block.
func writeMetaFile(blockID ulid.ULID, blockDir string, minT, maxT int64, stats tsdb.BlockStats, extendMeta func(tsdb.BlockMeta) interface{}) error {
	// If everything went fine, we have new TSDB block finished. The only missing thing is meta.json file.
	meta := tsdb.BlockMeta{
		ULID:    blockID,
		MinTime: minT,
		MaxTime: maxT + 1, // MaxT here is exclusive, so we add 1 to it.
		Stats:   stats,
		Compaction: tsdb.BlockMetaCompaction{
			Level:   1,
			Sources: []ulid.ULID{blockID},
		},
		Version: metaVersion1,
	}

	extendedMeta := extendMeta(meta)
	jsonMeta, err := json.MarshalIndent(extendedMeta, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	metaPath := filepath.Join(blockDir, "meta.json")
	if err := os.WriteFile(metaPath, jsonMeta, permFile); err != nil {
		return fmt.Errorf("failed to write %s: %w", metaPath, err)
	}
	return nil
}

func addSeriesToIndex(indexWriter *index.Writer, chunksWriter *chunks.Writer, seriesFiles []string, unsortedChunksReader *chunks.Reader) (stats tsdb.BlockStats, minT, maxT int64, outErr error) {
	minT = math.MaxInt64

	si, err := newSeriesIterator(seriesFiles)
	if err != nil {
		return stats, minT, maxT, err
	}
	defer func() {
		e := si.Close()
		if outErr == nil {
			outErr = e
		}
	}()

	ref := storage.SeriesRef(0)

	var ser series
	for ser, err = si.NextSeries(); err == nil; ser, err = si.NextSeries() {
		stats.NumSeries++

		// We need to load chunks for this series from unsortedChunksReader, and write them via chunksWriter
		for ix := range ser.Chunks {
			ser.Chunks[ix].Chunk, _, err = unsortedChunksReader.ChunkOrIterable(ser.Chunks[ix])
			if err != nil {
				return stats, minT, maxT, fmt.Errorf("failed to load chunk %d: %w", ser.Chunks[ix].Ref, err)
			}
			if ser.Chunks[ix].Chunk == nil {
				return stats, minT, maxT, fmt.Errorf("failed to load chunk %d: chunk is nil", ser.Chunks[ix].Ref)
			}
			ser.Chunks[ix].Ref = 0

			// Update stats
			stats.NumChunks++
			stats.NumSamples += uint64(ser.Chunks[ix].Chunk.NumSamples())
			if ser.Chunks[ix].MinTime < minT {
				minT = ser.Chunks[ix].MinTime
			}
			if ser.Chunks[ix].MaxTime > maxT {
				maxT = ser.Chunks[ix].MaxTime
			}
		}

		// Now write all chunks
		err = chunksWriter.WriteChunks(ser.Chunks...)
		if err != nil {
			return stats, minT, maxT, err
		}

		// After writing chunks, we have new Ref numbers. Check for that.
		for ix := range ser.Chunks {
			if ser.Chunks[ix].Ref == 0 {
				return stats, minT, maxT, fmt.Errorf("chunk reference not set after writing chunk")
			}
		}

		// Now we're ready to add series to TSDB index.
		ref++
		err = indexWriter.AddSeries(ref, ser.Metric, ser.Chunks...)
		if err != nil {
			return stats, minT, maxT, fmt.Errorf("failed to add series %v to index: %w", ser.Metric.String(), err)
		}
	}

	// We expect io.EOF from NextSeries.
	if !errors.Is(err, io.EOF) {
		return stats, minT, maxT, fmt.Errorf("io.EOF expected, got: %w", err)
	}
	return stats, minT, maxT, nil
}

func addSymbolsToIndexWriter(indexWriter *index.Writer, symbolFiles []string) error {
	si, err := newSymbolsIterator(symbolFiles)
	if err != nil {
		return err
	}

	var sym string
	for sym, err = si.NextSymbol(); err == nil; sym, err = si.NextSymbol() {
		err = indexWriter.AddSymbol(sym)
		if err != nil {
			break
		}
	}

	// io.EOF is reported after all symbols were read.
	if errors.Is(err, io.EOF) {
		err = nil
	}

	closeErr := si.Close()
	if err == nil {
		return closeErr
	}
	return err
}

func (b *Builder) addSeries(lbls labels.Labels, chunks []chunks.Meta) error {
	prev := labels.Label{}
	if err := lbls.Validate(func(l labels.Label) error {
		if prev.Name >= l.Name {
			return fmt.Errorf("labels names not unique or sorted: %v", lbls.String())
		}
		prev = l
		return nil
	}); err != nil {
		return err
	}

	for ix := range chunks {
		if chunks[ix].Ref == 0 {
			return fmt.Errorf("chunk reference not set")
		}

		// We don't want to store raw chunk into "series", only chunk reference.
		chunks[ix].Chunk = nil
	}

	b.seriesMtx.Lock()
	defer b.seriesMtx.Unlock()

	return b.series.addSeries(lbls, chunks)
}

// writeChunksForUnsortedSeries writes chunks to the disk, but also updates "Ref" field in chunks.Meta with
// chunk references.
func (b *Builder) writeChunksForUnsortedSeries(chunks []chunks.Meta) error {
	for ix := range chunks {
		chunks[ix].Ref = 0
	}

	b.chunksForUnsortedSeriesMtx.Lock()
	defer b.chunksForUnsortedSeriesMtx.Unlock()

	if err := b.chunksForUnsortedSeries.WriteChunks(chunks...); err != nil {
		return fmt.Errorf("failed to store chunks: %w", err)
	}

	return nil
}

func (b *Builder) addSymbols(lbls labels.Labels) error {
	b.symbolsMtx.Lock()
	defer b.symbolsMtx.Unlock()

	return lbls.Validate(func(l labels.Label) error {
		if err := b.symbols.addSymbol(l.Name); err != nil {
			return err
		}
		if err := b.symbols.addSymbol(l.Value); err != nil {
			return err
		}
		return nil
	})
}

// samplesToChunks iterates through samples, and stores them into XOR chunks, used by Prometheus TSDB.
// Samples must be ordered by timestamp, otherwise error is returned.
// If builder has MinBlockTime or MaxBlockTime set, samples outside of this time range will be ignored.
func samplesToChunks(samples chunkenc.Iterator, minBlockTime, maxBlockTime int64) ([]chunks.Meta, error) {
	metas := []chunks.Meta(nil)
	var (
		chunk *chunkenc.XORChunk
		meta  chunks.Meta
		app   chunkenc.Appender
	)

	// Finishes current chunk, and sets it to nil.
	finishChunk := func(maxTime int64) {
		chunk.Compact()
		meta.MaxTime = maxTime
		meta.Chunk = chunk
		metas = append(metas, meta)
		chunk = nil
		app = nil
	}

	prevTS := int64(0)
	for res := samples.Next(); res != chunkenc.ValNone; res = samples.Next() {
		if res != chunkenc.ValFloat {
			return nil, fmt.Errorf("non-float sample type: %s", res.String())
		}

		ts, val := samples.At()
		if ts <= prevTS {
			return nil, errors.Errorf("sample timestamps are not increasing, previous timestamp: %d, next timestamp: %d", prevTS, ts)
		}

		if minBlockTime != 0 && ts < minBlockTime {
			continue
		}
		if maxBlockTime != 0 && ts >= maxBlockTime {
			continue
		}

		// Start new chunk if needed.
		if chunk == nil {
			chunk = chunkenc.NewXORChunk()
			var err error
			app, err = chunk.Appender()
			if err != nil {
				panic(err)
			}

			meta = chunks.Meta{
				MinTime: ts,
			}
		}

		app.Append(0, ts, val)
		prevTS = ts

		if chunk.NumSamples() >= samplesPerChunk {
			finishChunk(ts)
		}
	}

	// If there is unfinished chunk, finish it too.
	if chunk != nil {
		finishChunk(prevTS)
	}

	return metas, samples.Err()
}

// CreateBlock uses supplied series with samples, and generates new TSDB block.
//
// CreateBlock creates new subdirectory for the block in supplied directory, and returns generated block ID.
// Block is only fully written if there was no returned error.
//
// It is safe to call CreateBlock multiple times (also concurrently) using the same directory. Each call will generate
// different block ID.
//
// Unlike Builder, CreateBlock uses only in-memory data, and assumes that series are already sorted by label.
func CreateBlock(ctx context.Context, series []storage.Series, dir string, extendMeta func(tsdb.BlockMeta) interface{}) (ulid.ULID, error) {
	blockID := ulid.MustNew(ulid.Now(), rand.Reader)

	blockDir := filepath.Join(dir, blockID.String())
	if err := os.MkdirAll(blockDir, permDir); err != nil {
		return blockID, fmt.Errorf("failed to create block directory %v: %w", blockDir, err)
	}

	// under temp, so that deleting temp will delete everything.
	chunksDir := filepath.Join(blockDir, "chunks")
	if err := os.MkdirAll(chunksDir, permDir); err != nil {
		return blockID, fmt.Errorf("failed to create chunks directory %v: %w", chunksDir, err)
	}

	closers := []io.Closer(nil)
	defer func() {
		// If there is anything to close here, there must already be error returned by the outer function.
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	// Prepare writers and readers.
	indexWriter, err := index.NewWriter(ctx, filepath.Join(blockDir, "index"))
	if err != nil {
		return blockID, err
	}
	closers = append(closers, indexWriter)

	err = addSortedInMemorySeriesSymbolsToIndex(indexWriter, series)
	if err != nil {
		return blockID, err
	}

	chunksWriter, err := chunks.NewWriter(chunksDir)
	if err != nil {
		return blockID, err
	}
	closers = append(closers, chunksWriter)

	stats, minT, maxT, err := addSortedInMemorySeriesToIndex(indexWriter, chunksWriter, series)
	if err != nil {
		return blockID, err
	}

	for len(closers) > 0 {
		c := closers[0]
		closers = closers[1:]

		if err := c.Close(); err != nil {
			return blockID, err
		}
	}

	return blockID, writeMetaFile(blockID, blockDir, minT, maxT, stats, extendMeta)
}

func addSortedInMemorySeriesToIndex(indexWriter *index.Writer, chunksWriter *chunks.Writer, series []storage.Series) (stats tsdb.BlockStats, minT, maxT int64, _ error) {
	ref := storage.SeriesRef(0)
	minT = math.MaxInt64

	var prevLbls labels.Labels
	for _, ser := range series {
		lbls := ser.Labels()
		if labels.Compare(prevLbls, lbls) >= 0 {
			return stats, minT, maxT, fmt.Errorf("series not sorted by labels: previous series: %s, next series: %s", prevLbls.String(), lbls.String())
		}

		stats.NumSeries++

		chks, err := samplesToChunks(ser.Iterator(nil), 0, 0)
		if err != nil {
			return stats, minT, maxT, err
		}

		if err := chunksWriter.WriteChunks(chks...); err != nil {
			return stats, minT, maxT, err
		}

		for ix := range chks {
			// After writing chunks, we have new Ref numbers. Check for that.
			if chks[ix].Ref == 0 {
				return stats, minT, maxT, fmt.Errorf("chunk reference not set after writing chunk")
			}

			// Update stats.
			stats.NumChunks++
			stats.NumSamples += uint64(chks[ix].Chunk.NumSamples())
			if chks[ix].MinTime < minT {
				minT = chks[ix].MinTime
			}
			if chks[ix].MaxTime > maxT {
				maxT = chks[ix].MaxTime
			}
		}

		// Now we're ready to add series to TSDB index.
		ref++
		if err := indexWriter.AddSeries(ref, ser.Labels(), chks...); err != nil {
			return stats, minT, maxT, fmt.Errorf("failed to add series %v to index: %w", lbls.String(), err)
		}
	}

	return stats, minT, maxT, nil
}

func addSortedInMemorySeriesSymbolsToIndex(indexWriter *index.Writer, series []storage.Series) error {
	symbolsMap := map[string]struct{}{}
	symbolsMap[""] = struct{}{}

	for _, s := range series {
		s.Labels().Range(func(l labels.Label) {
			symbolsMap[l.Name] = struct{}{}
			symbolsMap[l.Value] = struct{}{}
		})
	}

	// collect to slice
	symbols := make([]string, 0, len(symbolsMap))
	for s := range symbolsMap {
		symbols = append(symbols, s)
	}

	sort.Strings(symbols)

	for _, s := range symbols {
		err := indexWriter.AddSymbol(s)
		if err != nil {
			return err
		}
	}
	return nil
}
