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

func (m *LogsModel) ToggleStream(stream runlog.StreamKind) {
	for index, current := range m.Streams {
		if current == stream {
			m.Streams = append(m.Streams[:index], m.Streams[index+1:]...)
			return
		}
	}
	m.Streams = append(m.Streams, stream)
}

func (m *LogsModel) Clear() {
	m.Records = nil
	m.VisibleCount = 0
	m.bufferedBytes = 0
	m.Dropped = 0
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
	records := m.filteredRecords()
	_, _ = fmt.Fprintf(&output, "%s  %s  %s  %s  %s  %s\n\n",
		accentStyle.Render("services:"+joinOrAll(m.Services)),
		accentStyle.Render("streams:"+streamNames(m.Streams)),
		stateStyle(onOff(m.Follow)).Render("follow:"+onOff(m.Follow)),
		warningStyle.Render(fmt.Sprintf("paused:%t", m.Paused)),
		mutedStyle.Render(fmt.Sprintf("wrap:%t", m.Wrap)),
		mutedStyle.Render(fmt.Sprintf("visible:%d dropped:%d", len(records), m.Dropped)),
	)
	limit := max(1, height-5)
	start := max(0, len(records)-limit)
	for _, record := range records[start:] {
		text := sanitizeLogText(record.Text)
		if !m.Wrap {
			text = truncate(text, 100)
		}
		stream := stateStyle(string(record.Stream)).Render(fmt.Sprintf("%-7s", record.Stream))
		_, _ = fmt.Fprintf(&output, "%s %-10s %s %s\n",
			mutedStyle.Render(record.Time.Format("15:04:05.000")), record.Service, stream, text)
	}
	output.WriteString("\n" + renderKey("p", "pause") + "  " + renderKey("f", "follow") +
		"  " + renderKey("w", "wrap") + "  " + renderKey("o/e", "streams") +
		"  " + renderKey("/", "search") + "  " + renderKey("x", "clear"))
	return output.String()
}

func (m LogsModel) filteredRecords() []runlog.LogRecord {
	end := min(m.VisibleCount, len(m.Records))
	records := make([]runlog.LogRecord, 0, end)
	for _, record := range m.Records[:end] {
		if len(m.Services) > 0 && !containsString(m.Services, record.Service) {
			continue
		}
		if len(m.Streams) > 0 && !containsStream(m.Streams, record.Stream) {
			continue
		}
		text := sanitizeLogText(record.Text)
		if m.Search != "" && !strings.Contains(text, m.Search) {
			continue
		}
		records = append(records, record)
	}
	return records
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func containsStream(values []runlog.StreamKind, candidate runlog.StreamKind) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
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
