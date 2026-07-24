package runstate

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type atomicWriteHooks struct {
	write    func(*os.File, []byte) (int, error)
	syncFile func(*os.File) error
	rename   func(string, string) error
	syncDir  func(*os.File) error
}

// WriteJSONAtomic replaces path with an indented JSON document using a
// same-directory temporary file, fsync, rename, and directory fsync.
func WriteJSONAtomic(path string, value any, mode os.FileMode) error {
	return writeJSONAtomic(path, value, mode, atomicWriteHooks{})
}

func writeJSONAtomic(path string, value any, mode os.FileMode, hooks atomicWriteHooks) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal JSON artifact")
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.Wrap(err, "create JSON artifact directory")
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return errors.Wrap(err, "create JSON artifact temporary file")
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(mode); err != nil {
		return errors.Wrap(err, "set JSON artifact temporary mode")
	}
	write := hooks.write
	if write == nil {
		write = func(file *os.File, contents []byte) (int, error) {
			return file.Write(contents)
		}
	}
	n, err := write(tmp, data)
	if err != nil {
		return errors.Wrap(err, "write JSON artifact temporary file")
	}
	if n != len(data) {
		return errors.Wrap(io.ErrShortWrite, "write JSON artifact temporary file")
	}
	syncFile := hooks.syncFile
	if syncFile == nil {
		syncFile = func(file *os.File) error {
			return file.Sync()
		}
	}
	if err := syncFile(tmp); err != nil {
		return errors.Wrap(err, "sync JSON artifact temporary file")
	}
	if err := tmp.Close(); err != nil {
		return errors.Wrap(err, "close JSON artifact temporary file")
	}
	closed = true

	rename := hooks.rename
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(tmpPath, path); err != nil {
		return errors.Wrap(err, "replace JSON artifact")
	}

	dirHandle, err := os.Open(dir)
	if err != nil {
		return errors.Wrap(err, "open JSON artifact directory")
	}
	defer func() { _ = dirHandle.Close() }()
	syncDir := hooks.syncDir
	if syncDir == nil {
		syncDir = func(file *os.File) error {
			return file.Sync()
		}
	}
	if err := syncDir(dirHandle); err != nil {
		return errors.Wrap(err, "sync JSON artifact directory")
	}
	return nil
}
