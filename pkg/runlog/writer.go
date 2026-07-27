package runlog

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	RecordVersion   = 1
	JournalFileName = "logs.jsonl"
)

type CaptureOptions struct {
	RunID          string
	Service        string
	MaxRecordBytes int
	Now            func() time.Time
}

type CaptureStream struct {
	Kind StreamKind
	Read io.Reader
	Raw  io.Writer
}

type syncWriter interface {
	Sync() error
}

type capturedFrame struct {
	stream StreamKind
	frame  Frame
}

func Capture(
	ctx context.Context,
	options CaptureOptions,
	journal io.Writer,
	streams ...CaptureStream,
) error {
	if options.RunID == "" || options.Service == "" {
		return errors.New("capture requires run ID and service")
	}
	if journal == nil {
		return errors.New("capture requires journal writer")
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	captureContext, cancel := context.WithCancel(ctx)
	defer cancel()
	group, groupContext := errgroup.WithContext(captureContext)
	frames := make(chan capturedFrame, 64)
	readerFailure := make(chan error, len(streams))
	for _, stream := range streams {
		stream := stream
		if stream.Read == nil || stream.Raw == nil {
			return errors.New("capture stream requires reader and raw writer")
		}
		group.Go(func() error {
			err := captureStream(groupContext, stream, options.MaxRecordBytes, frames)
			if err != nil {
				readerFailure <- err
			}
			return err
		})
	}
	readersDone := make(chan error, 1)
	go func() {
		readersDone <- group.Wait()
		close(frames)
	}()

	buffered := bufio.NewWriterSize(journal, 64*1024)
	encoder := json.NewEncoder(buffered)
	var sequence uint64
	for {
		select {
		case readerErr := <-readerFailure:
			cancel()
			closeCaptureReaders(streams)
			<-readersDone
			return readerErr
		case frame, ok := <-frames:
			if !ok {
				if err := <-readersDone; err != nil {
					return err
				}
				if err := buffered.Flush(); err != nil {
					return errors.Wrap(err, "flush structured log journal")
				}
				if syncer, ok := journal.(syncWriter); ok {
					if err := syncer.Sync(); err != nil {
						return errors.Wrap(err, "sync structured log journal")
					}
				}
				for _, stream := range streams {
					if syncer, ok := stream.Raw.(syncWriter); ok {
						if err := syncer.Sync(); err != nil {
							return errors.Wrap(err, "sync raw log")
						}
					}
				}
				return nil
			}
			sequence++
			record := LogRecord{
				Version:  RecordVersion,
				RunID:    options.RunID,
				Sequence: sequence,
				Time:     now(),
				Source:   SourceService,
				Service:  options.Service,
				Stream:   frame.stream,
				Text:     frame.frame.Text,
				Partial:  frame.frame.Partial,
			}
			if err := encoder.Encode(record); err != nil {
				cancel()
				closeCaptureReaders(streams)
				<-readersDone
				return errors.Wrap(err, "write structured log record")
			}
			if err := buffered.Flush(); err != nil {
				cancel()
				closeCaptureReaders(streams)
				<-readersDone
				return errors.Wrap(err, "flush structured log record")
			}
		}
	}
}

func captureStream(
	ctx context.Context,
	stream CaptureStream,
	maxRecordBytes int,
	frames chan<- capturedFrame,
) error {
	framer := NewFramer(maxRecordBytes)
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := stream.Read.Read(buffer)
		if count > 0 {
			data := buffer[:count]
			if err := writeAll(stream.Raw, data); err != nil {
				return errors.Wrap(err, "write raw log")
			}
			for _, frame := range framer.Write(data) {
				select {
				case frames <- capturedFrame{stream: stream.Kind, frame: frame}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				return errors.Wrap(readErr, "read service log pipe")
			}
			for _, frame := range framer.Close() {
				select {
				case frames <- capturedFrame{stream: stream.Kind, frame: frame}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		}
	}
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		count, err := writer.Write(data)
		if err != nil {
			return err
		}
		if count <= 0 {
			return io.ErrShortWrite
		}
		data = data[count:]
	}
	return nil
}

func closeCaptureReaders(streams []CaptureStream) {
	for _, stream := range streams {
		if closer, ok := stream.Read.(io.Closer); ok {
			_ = closer.Close()
		}
	}
}
