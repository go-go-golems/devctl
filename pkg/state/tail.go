package state

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
)

func TailLines(path string, tailLines int, maxBytes int64) ([]string, error) {
	if path == "" {
		return nil, errors.New("missing path")
	}
	if tailLines <= 0 {
		tailLines = 20
	}
	if maxBytes <= 0 {
		maxBytes = 2 << 20
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "open")
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			// Best-effort close; no caller-visible action.
			_ = cerr
		}
	}()

	info, err := f.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "stat")
	}
	size := info.Size()
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "seek")
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, errors.Wrap(err, "read")
	}
	if start > 0 {
		if i := bytes.IndexByte(b, '\n'); i >= 0 && i+1 < len(b) {
			b = b[i+1:]
		}
	}

	lines := strings.Split(string(b), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > tailLines {
		lines = append([]string{}, lines[len(lines)-tailLines:]...)
	}
	return lines, nil
}
