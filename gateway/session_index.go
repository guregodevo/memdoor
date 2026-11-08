package gateway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// A transcript's index: where each record's line is in the file and what the
// trim rule needs to know about it. With it a save writes the file the old
// append-then-compact pair wrote, in one copy-and-rename, without parsing the
// transcript again: past 1 MiB that pair parsed and rewrote the kept tail on
// every save (#197).
type transcriptIndex struct {
	size    int64
	modTime time.Time
	entries []indexEntry
}

type indexEntry struct {
	off, n int64
	mark   recordMark
	// line is the record as the compaction rewrite writes it, when the bytes
	// on disk differ (an older writer, unknown fields, no final newline).
	line []byte
}

// recordMark is what trimPoint reads of a record.
type recordMark struct{ boundary, turn bool }

func markOf(rec MessageRecord) recordMark {
	return recordMark{boundary: rec.Boundary, turn: startsATurn(rec)}
}

// sessionIndexCap bounds the indexes kept in memory; one is rebuilt by a
// single read when its transcript is saved again.
const sessionIndexCap = 64

// sessionLineCap matches readAllRecords: a longer line fails a read there.
const sessionLineCap = 1024 * 1024

var errNoIndex = errors.New("transcript cannot be indexed")

// indexFor returns the index of path, rebuilt when the file changed under
// it. A missing file has an empty index. MUST be called with sp.mu held.
func (sp *SessionPersistence) indexFor(path string) (*transcriptIndex, error) {
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		delete(sp.indexes, path)
		return &transcriptIndex{}, nil
	}
	if err != nil {
		return nil, err
	}
	if idx, ok := sp.indexes[path]; ok && idx.size == fi.Size() && idx.modTime.Equal(fi.ModTime()) {
		return idx, nil
	}
	idx, err := buildTranscriptIndex(path)
	if err != nil {
		delete(sp.indexes, path)
		return nil, err
	}
	sp.keepIndex(path, idx)
	return idx, nil
}

func (sp *SessionPersistence) keepIndex(path string, idx *transcriptIndex) {
	if _, ok := sp.indexes[path]; !ok && len(sp.indexes) >= sessionIndexCap {
		for other := range sp.indexes {
			delete(sp.indexes, other)
			break
		}
	}
	sp.indexes[path] = idx
}

func (sp *SessionPersistence) forgetIndex(path string) { delete(sp.indexes, path) }

// buildTranscriptIndex reads path once. Lines readAllRecords skips (empty,
// malformed) are no entry. A file whose last line has no newline is not
// indexed: appending glues the batch's first record to it, which only the
// old path reproduces.
func buildTranscriptIndex(path string) (*transcriptIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	idx := &transcriptIndex{size: fi.Size(), modTime: fi.ModTime()}
	r := bufio.NewReader(f)
	var off int64
	for {
		raw, err := r.ReadBytes('\n')
		if len(raw) > 0 {
			if !bytes.HasSuffix(raw, []byte{'\n'}) {
				return nil, fmt.Errorf("%w: last line has no newline", errNoIndex)
			}
			body := bytes.TrimSuffix(raw, []byte{'\n'})
			if len(bytes.TrimSuffix(body, []byte{'\r'})) >= sessionLineCap {
				return nil, fmt.Errorf("%w: a line over %d bytes", errNoIndex, sessionLineCap)
			}
			e, ok, lineErr := indexLine(body, off, int64(len(raw)))
			if lineErr != nil {
				return nil, lineErr
			}
			if ok {
				idx.entries = append(idx.entries, e)
			}
			off += int64(len(raw))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if off != idx.size {
		return nil, fmt.Errorf("%w: read %d of %d bytes", errNoIndex, off, idx.size)
	}
	return idx, nil
}

// indexLine is the entry of one line, false for a line the reader skips. A
// record that does not marshal again fails the index: the compaction rewrite
// fails on it, which only the old path reproduces.
func indexLine(body []byte, off, n int64) (indexEntry, bool, error) {
	if len(bytes.TrimSuffix(body, []byte{'\r'})) == 0 {
		return indexEntry{}, false, nil
	}
	var rec MessageRecord
	if json.Unmarshal(body, &rec) != nil {
		return indexEntry{}, false, nil
	}
	again, err := recordLine(rec)
	if err != nil {
		return indexEntry{}, false, fmt.Errorf("%w: %w", errNoIndex, err)
	}
	e := indexEntry{off: off, n: n, mark: markOf(rec)}
	if !bytes.Equal(again[:len(again)-1], body) {
		e.line = again
	}
	return e, true, nil
}

// writeBatch saves recs to path, and when compact is set trims the file the
// way compactSessionFile does, in one copy-and-rename. A transcript it cannot
// index takes the old path. MUST be called with sp.mu held. Returns the
// marshaled byte count, as marshalAndWriteRecord counts it.
func (sp *SessionPersistence) writeBatch(path, sessionID string, recs []MessageRecord, compact bool) (int, error) {
	lines := make([][]byte, len(recs))
	marks := make([]recordMark, len(recs))
	total := 0
	for i, rec := range recs {
		line, err := recordLine(rec)
		if err != nil {
			return 0, err
		}
		lines[i], marks[i] = line, markOf(rec)
		total += len(line) - 1
	}

	idx, err := sp.indexFor(path)
	if err != nil {
		sp.forgetIndex(path)
		return total, sp.writeBatchUnindexed(path, sessionID, recs, compact)
	}

	physical := idx.size
	for _, l := range lines {
		physical += int64(len(l))
	}
	cut := 0
	if compact && physical >= sessionCompactTriggerBytes {
		all := make([]recordMark, 0, len(idx.entries)+len(marks))
		for _, e := range idx.entries {
			all = append(all, e.mark)
		}
		cut = trimMarks(append(all, marks...))
	}
	next, err := sp.rewriteTranscript(path, idx, lines, marks, cut)
	if err != nil && cut > 0 {
		// The old pair saved the batch before it tried the trim, and a trim
		// that failed left the batch saved.
		sp.log.Warn("session file compaction failed (file still usable)",
			slog.String("session", sessionID), slog.String("error", err.Error()))
		next, err = sp.rewriteTranscript(path, idx, lines, marks, 0)
		cut = 0
	}
	if err != nil {
		sp.forgetIndex(path)
		return 0, err
	}
	sp.keepIndex(path, next)
	if cut > 0 {
		sp.log.Info("compacted session file",
			slog.String("session", sessionID),
			slog.Int("kept", len(next.entries)), slog.Int("dropped", cut))
	}
	return total, nil
}

// writeBatchUnindexed is the save as it was before the index: append, then
// compact by parsing the file.
func (sp *SessionPersistence) writeBatchUnindexed(path, sessionID string, recs []MessageRecord, compact bool) error {
	err := appendSessionBatch(path, func(file io.Writer) error {
		for _, rec := range recs {
			if _, err := marshalAndWriteRecord(file, rec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil || !compact {
		return err
	}
	if err := sp.compactSessionFile(path, sessionID); err != nil {
		sp.log.Warn("session file compaction failed (file still usable)",
			slog.String("session", sessionID), slog.String("error", err.Error()))
	}
	return nil
}

// rewriteTranscript writes, to a temp file renamed over path, the records of
// idx and then lines, from record cut on (cut 0: the old bytes as they are,
// skipped lines included). It returns the index of what it wrote.
func (sp *SessionPersistence) rewriteTranscript(path string, idx *transcriptIndex, lines [][]byte, marks []recordMark, cut int) (*transcriptIndex, error) {
	next := &transcriptIndex{}
	err := replaceSessionFile(path, func(tmp io.Writer) error {
		w := &countingWriter{w: tmp}
		var src *os.File
		if idx.size > 0 {
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("read transcript: %w", err)
			}
			defer f.Close()
			src = f
		}
		if cut == 0 && src != nil {
			if err := copySpan(w, src, 0, idx.size); err != nil {
				return err
			}
			next.entries = append(next.entries, idx.entries...)
		}
		for i, e := range idx.entries {
			if cut == 0 || i < cut {
				continue
			}
			kept := indexEntry{off: w.n, mark: e.mark}
			if e.line != nil {
				if err := writeAll(w, e.line); err != nil {
					return err
				}
			} else if err := copySpan(w, src, e.off, e.n); err != nil {
				return err
			}
			kept.n = w.n - kept.off
			next.entries = append(next.entries, kept)
		}
		for i, l := range lines {
			if len(idx.entries)+i < cut {
				continue
			}
			e := indexEntry{off: w.n, n: int64(len(l)), mark: marks[i]}
			if err := writeAll(w, l); err != nil {
				return err
			}
			next.entries = append(next.entries, e)
		}
		next.size = w.n
		return nil
	})
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() != next.size {
		return nil, fmt.Errorf("transcript is %d bytes after a save that wrote %d", fi.Size(), next.size)
	}
	next.modTime = fi.ModTime()
	return next, nil
}

// replaceSessionFile writes a new transcript beside path and renames it over
// path once it is written, synced and closed: a failure or a crash leaves the
// old file whole.
func replaceSessionFile(path string, write func(io.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-save-*")
	if err != nil {
		return fmt.Errorf("create transcript checkpoint: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := write(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync transcript checkpoint: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close transcript checkpoint: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("commit transcript checkpoint: %w", err)
	}
	return nil
}

func copySpan(w io.Writer, src io.ReaderAt, off, n int64) error {
	copied, err := io.Copy(w, io.NewSectionReader(src, off, n))
	if err != nil {
		return fmt.Errorf("copy transcript: %w", err)
	}
	if copied != n {
		return fmt.Errorf("copy transcript: %w", io.ErrUnexpectedEOF)
	}
	return nil
}

func writeAll(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
