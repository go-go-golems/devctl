package runlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

type lineCountingWriter struct {
	lines int
}

func (w *lineCountingWriter) Write(data []byte) (int, error) {
	w.lines += bytes.Count(data, []byte{'\n'})
	return len(data), nil
}

func TestFramerNormalizesTextWithoutChangingRawCapture(t *testing.T) {
	framer := NewFramer(4)
	frames := append(framer.Write([]byte("a\r\n\n123456\nlast")), framer.Close()...)
	want := []Frame{
		{Text: "a"},
		{Text: ""},
		{Text: "1234", Partial: true},
		{Text: "56"},
		{Text: "last", Partial: true},
	}
	if len(frames) != len(want) {
		t.Fatalf("frames = %#v, want %#v", frames, want)
	}
	for index := range want {
		if frames[index] != want[index] {
			t.Fatalf("frame[%d] = %#v, want %#v", index, frames[index], want[index])
		}
	}
}

func TestCaptureProducesUniqueSequenceAndExactRawBytes(t *testing.T) {
	stdout := "one\r\ntwo-without-newline"
	stderr := "error\n"
	var stdoutRaw bytes.Buffer
	var stderrRaw bytes.Buffer
	var journal bytes.Buffer
	timestamp := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	err := Capture(context.Background(), CaptureOptions{
		RunID: "run-1", Service: "web", MaxRecordBytes: 8,
		Now: func() time.Time { return timestamp },
	}, &journal,
		CaptureStream{Kind: StreamStdout, Read: strings.NewReader(stdout), Raw: &stdoutRaw},
		CaptureStream{Kind: StreamStderr, Read: strings.NewReader(stderr), Raw: &stderrRaw},
	)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if stdoutRaw.String() != stdout || stderrRaw.String() != stderr {
		t.Fatalf("raw capture changed: stdout=%q stderr=%q", stdoutRaw.String(), stderrRaw.String())
	}
	lines := bytes.Split(bytes.TrimSpace(journal.Bytes()), []byte{'\n'})
	seen := map[uint64]struct{}{}
	for index, line := range lines {
		var record LogRecord
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode record %d: %v", index, err)
		}
		if record.Sequence != uint64(index+1) {
			t.Fatalf("sequence[%d] = %d", index, record.Sequence)
		}
		if _, duplicate := seen[record.Sequence]; duplicate {
			t.Fatalf("duplicate sequence %d", record.Sequence)
		}
		seen[record.Sequence] = struct{}{}
	}
}

func TestCaptureFailureClosesBlockingPeerReader(t *testing.T) {
	failingReader, failingInput := io.Pipe()
	blockingReader, blockingInput := io.Pipe()
	defer func() { _ = failingInput.Close() }()
	defer func() { _ = blockingInput.Close() }()
	done := make(chan error, 1)
	go func() {
		done <- Capture(context.Background(), CaptureOptions{
			RunID: "run-1", Service: "web",
		}, io.Discard,
			CaptureStream{Kind: StreamStdout, Read: failingReader, Raw: failingWriter{}},
			CaptureStream{Kind: StreamStderr, Read: blockingReader, Raw: io.Discard},
		)
	}()
	if _, err := failingInput.Write([]byte("trigger\n")); err != nil {
		t.Fatalf("feed failing stream: %v", err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "disk full") {
			t.Fatalf("capture error = %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("capture failure waited for blocking peer reader")
	}
}

func TestCaptureOneHundredThousandRecordsUsesBoundedPipeline(t *testing.T) {
	const recordsPerStream = 50_000
	journal := &lineCountingWriter{}
	err := Capture(context.Background(), CaptureOptions{
		RunID: "run-1", Service: "load",
	}, journal,
		CaptureStream{
			Kind: StreamStdout, Read: strings.NewReader(strings.Repeat("o\n", recordsPerStream)),
			Raw: io.Discard,
		},
		CaptureStream{
			Kind: StreamStderr, Read: strings.NewReader(strings.Repeat("e\n", recordsPerStream)),
			Raw: io.Discard,
		},
	)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if journal.lines != 2*recordsPerStream {
		t.Fatalf("journal records = %d, want %d", journal.lines, 2*recordsPerStream)
	}
}

func TestCapturePreservesInvalidUTF8AndANSIInRawBytes(t *testing.T) {
	input := []byte{0xff, ' ', 0x1b, '[', '3', '1', 'm', 'x', 0x1b, '[', '0', 'm', '\n'}
	var raw bytes.Buffer
	var journal bytes.Buffer
	err := Capture(context.Background(), CaptureOptions{
		RunID: "run-1", Service: "web",
	}, &journal, CaptureStream{
		Kind: StreamStdout, Read: bytes.NewReader(input), Raw: &raw,
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if !bytes.Equal(raw.Bytes(), input) {
		t.Fatalf("raw bytes changed: %x != %x", raw.Bytes(), input)
	}
	var record LogRecord
	if err := json.Unmarshal(bytes.TrimSpace(journal.Bytes()), &record); err != nil {
		t.Fatalf("decode journal: %v", err)
	}
	if !strings.Contains(record.Text, "\u001b[31m") || !strings.Contains(record.Text, "\ufffd") {
		t.Fatalf("journal text lost ANSI or UTF-8 replacement: %q", record.Text)
	}
}

func FuzzFramerNeverExceedsLimit(f *testing.F) {
	f.Add([]byte("one\ntwo\r\nthree"))
	f.Fuzz(func(t *testing.T, data []byte) {
		const limit = 17
		framer := NewFramer(limit)
		frames := append(framer.Write(data), framer.Close()...)
		for _, frame := range frames {
			if len([]byte(frame.Text)) > limit {
				t.Fatalf("frame length %d exceeds %d", len([]byte(frame.Text)), limit)
			}
		}
	})
}
