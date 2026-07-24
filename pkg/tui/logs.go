package tui

import (
	"fmt"
	"strings"

	"github.com/go-go-golems/devctl/pkg/runlog"
)

const (
	defaultLogRecordLimit = 2000
	defaultLogByteLimit   = 2 * 1024 * 1024
)

type LogsModel struct {
	Records       []runlog.LogRecord
	Services      []string
	Streams       []runlog.StreamKind
	Search        string
	Follow        bool
	Paused        bool
	Wrap          bool
	Dropped       uint64
	VisibleCount  int
	MaxRecords    int
	MaxBytes      int
	bufferedBytes int
}

func NewLogsModel() LogsModel {
	return LogsModel{
		Streams: []runlog.StreamKind{runlog.StreamStdout, runlog.StreamStderr},
		Follow:  true, MaxRecords: defaultLogRecordLimit, MaxBytes: defaultLogByteLimit,
	}
}

func (m *LogsModel) Add(record runlog.LogRecord) {
	m.Records = append(m.Records, record)
	m.bufferedBytes += len(record.Text)
	if !m.Paused {
		m.VisibleCount = len(m.Records)
	}
	for len(m.Records) > m.MaxRecords || m.bufferedBytes > m.MaxBytes {
		if len(m.Records) == 0 {
			break
		}
		m.bufferedBytes -= len(m.Records[0].Text)
		m.Records = m.Records[1:]
		m.Dropped++
		if m.VisibleCount > 0 {
			m.VisibleCount--
		}
	}
}

func (m *LogsModel) TogglePause() {
	m.Paused = !m.Paused
	if !m.Paused {
		m.VisibleCount = len(m.Records)
	}
}

func (m LogsModel) View(height int) string {
	var output strings.Builder
	_, _ = fmt.Fprintf(
		&output, "Filters: services=%s streams=%s follow=%s paused=%t wrap=%t dropped=%d\n\n",
		joinOrAll(m.Services), streamNames(m.Streams), onOff(m.Follow), m.Paused, m.Wrap, m.Dropped,
	)
	limit := max(1, height-5)
	end := min(m.VisibleCount, len(m.Records))
	start := max(0, end-limit)
	for _, record := range m.Records[start:end] {
		text := sanitizeLogText(record.Text)
		if m.Search != "" && !strings.Contains(text, m.Search) {
			continue
		}
		if !m.Wrap {
			text = truncate(text, 100)
		}
		_, _ = fmt.Fprintf(
			&output, "%s %-10s %-7s %s\n",
			record.Time.Format("15:04:05.000"), record.Service, record.Stream, text,
		)
	}
	output.WriteString("\n[p] pause  [f] follow  [w] wrap  [/] search")
	return output.String()
}

func sanitizeLogText(value string) string {
	var output strings.Builder
	output.Grow(len(value))
	state := 0
	for _, character := range value {
		switch state {
		case 0:
			switch {
			case character == '\x1b':
				state = 1
			case character == '\t' || character >= ' ':
				output.WriteRune(character)
			}
		case 1:
			if character == '[' {
				state = 2
			} else {
				state = 0
			}
		case 2:
			if character >= '@' && character <= '~' {
				state = 0
			}
		}
	}
	return output.String()
}

func joinOrAll(values []string) string {
	if len(values) == 0 {
		return "all"
	}
	return strings.Join(values, ",")
}

func streamNames(values []runlog.StreamKind) string {
	names := make([]string, 0, len(values))
	for _, value := range values {
		names = append(names, string(value))
	}
	return joinOrAll(names)
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}
