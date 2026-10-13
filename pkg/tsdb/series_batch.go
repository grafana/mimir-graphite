package tsdb

import (
	"container/heap"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunks"

	promErrors "github.com/prometheus/prometheus/tsdb/errors"
)

type series struct {
	Metric labels.Labels
	Chunks []chunks.Meta // Note: we clear the Chunk field before serialization.
}

// gobSeries is the on-disk form of series. labels.Labels has no exported fields, so Gob can't encode it directly.
type gobSeries struct {
	Metric []labels.Label
	Chunks []chunks.Meta
}

// seriesBatcher keeps list of series in memory, until and then stores them sorted into files.
type seriesBatcher struct {
	dir   string
	limit int

	files  []string // paths of series files, which were sent to flushers for flushing
	buffer []series
}

func newSeriesBatcher(limit int, dir string) *seriesBatcher {
	return &seriesBatcher{
		limit:  limit,
		dir:    dir,
		buffer: make([]series, 0, limit),
	}
}

func (sb *seriesBatcher) addSeries(lbls labels.Labels, chunks []chunks.Meta) error {
	// TODO: sort and validate labels

	sb.buffer = append(sb.buffer, series{
		Metric: lbls,
		Chunks: chunks,
	})
	return sb.flushSeries(false)
}

func (sb *seriesBatcher) flushSeries(force bool) error {
	if !force && len(sb.buffer) < sb.limit {
		return nil
	}

	if len(sb.buffer) == 0 {
		return nil
	}

	seriesFile := filepath.Join(sb.dir, fmt.Sprintf("series_%d", len(sb.files)))
	sb.files = append(sb.files, seriesFile)

	sortedSeries := sb.buffer
	sort.Slice(sortedSeries, func(i, j int) bool {
		return labels.Compare(sortedSeries[i].Metric, sortedSeries[j].Metric) < 0
	})

	sb.buffer = make([]series, 0, sb.limit)
	return writeSeriesToFile(seriesFile, sortedSeries)
}

// getSymbolFiles returns list of symbol files used to flush symbols to. Only valid if there were no errors.
func (sb *seriesBatcher) getSeriesFiles() []string {
	return sb.files
}

func writeSeriesToFile(filename string, sortedSeries []series) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	// Snappy is used for buffering and to create smaller files.
	sn := snappy.NewBufferedWriter(f)
	enc := gob.NewEncoder(sn)

	errs := promErrors.NewMulti()

	for _, s := range sortedSeries {
		gs := gobSeries{Metric: make([]labels.Label, 0, s.Metric.Len()), Chunks: s.Chunks}
		s.Metric.Range(func(l labels.Label) { gs.Metric = append(gs.Metric, l) })
		err := enc.Encode(gs)
		if err != nil {
			errs.Add(err)
			break
		}
	}

	errs.Add(sn.Close())
	errs.Add(f.Close())
	return errs.Err()
}

// Implements heap.Interface using symbols from files.
type seriesHeap []*seriesFile

// Len implements sort.Interface.
func (s *seriesHeap) Len() int {
	return len(*s)
}

// Less implements sort.Interface.
func (s *seriesHeap) Less(i, j int) bool {
	iw, ierr := (*s)[i].Peek()
	if ierr != nil {
		// Empty labels will be sorted first, so error will be returned before any other result.
		iw = series{}
	}

	jw, jerr := (*s)[j].Peek()
	if jerr != nil {
		jw = series{}
	}

	return labels.Compare(iw.Metric, jw.Metric) < 0
}

// Swap implements sort.Interface.
func (s *seriesHeap) Swap(i, j int) {
	(*s)[i], (*s)[j] = (*s)[j], (*s)[i]
}

// Push implements heap.Interface. Push should add x as element Len().
func (s *seriesHeap) Push(x interface{}) {
	*s = append(*s, x.(*seriesFile))
}

// Pop implements heap.Interface. Pop should remove and return element Len() - 1.
func (s *seriesHeap) Pop() interface{} {
	l := len(*s)
	res := (*s)[l-1]
	*s = (*s)[:l-1]
	return res
}

type seriesIterator struct {
	files []*os.File
	heap  seriesHeap

	// We remember last returned labels, to detect duplicate or out-of-order series.
	lastReturnedLabels labels.Labels
}

func newSeriesIterator(filenames []string) (*seriesIterator, error) {
	files, err := openFiles(filenames)
	if err != nil {
		return nil, err
	}

	var serFiles []*seriesFile
	for _, f := range files {
		serFiles = append(serFiles, newSeriesFile(f))
	}

	h := &seriesIterator{
		files: files,
		heap:  serFiles,
	}

	heap.Init(&h.heap)

	return h, nil
}

// NextSeries advances iterator forward, and returns next series (in sorted-labels order).
// If there is no next element, returns err == io.EOF.
func (sit *seriesIterator) NextSeries() (series, error) {
	for len(sit.heap) > 0 {
		result, err := sit.heap[0].Next()
		if errors.Is(err, io.EOF) {
			// End of file, remove it from heap, and try next file.
			heap.Remove(&sit.heap, 0)
			continue
		}

		if err != nil {
			return series{}, err
		}

		heap.Fix(&sit.heap, 0)

		if labels.Compare(sit.lastReturnedLabels, result.Metric) >= 0 {
			// Cannot really be out-of-order, because we take "lowest" series from the heap. But it can be duplicate.
			return series{}, fmt.Errorf("duplicate series: %s, %s", sit.lastReturnedLabels.String(), result.Metric.String())
		}

		sit.lastReturnedLabels = result.Metric
		return result, nil
	}

	return series{}, io.EOF
}

// Close all files.
func (sit *seriesIterator) Close() error {
	errs := promErrors.NewMulti()
	for _, f := range sit.files {
		errs.Add(f.Close())
	}
	return errs.Err()
}

type seriesFile struct {
	dec *gob.Decoder

	nextValid  bool // if true, nextSeries and nextErr have the next series
	nextSeries series
	nextErr    error
}

func newSeriesFile(f *os.File) *seriesFile {
	sn := snappy.NewReader(f)
	dec := gob.NewDecoder(sn)

	return &seriesFile{
		dec: dec,
	}
}

// Peek returns next symbol or error, but also preserves them for subsequent Peek or Next calls.
func (sf *seriesFile) Peek() (series, error) {
	if sf.nextValid {
		return sf.nextSeries, sf.nextErr
	}

	sf.nextValid = true
	sf.nextSeries, sf.nextErr = sf.readNext()
	return sf.nextSeries, sf.nextErr
}

// Next advances iterator and returns the next symbol or error.
func (sf *seriesFile) Next() (series, error) {
	if sf.nextValid {
		defer func() {
			sf.nextValid = false
			sf.nextSeries = series{}
			sf.nextErr = nil
		}()
		return sf.nextSeries, sf.nextErr
	}

	return sf.readNext()
}

func (sf *seriesFile) readNext() (series, error) {
	var gs gobSeries
	err := sf.dec.Decode(&gs)
	// Decode returns io.EOF at the end.
	if err != nil {
		return series{}, err
	}

	return series{Metric: labels.New(gs.Metric...), Chunks: gs.Chunks}, nil
}
