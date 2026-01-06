package models

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/tui"
)

type LogStream string

const (
	LogStdout LogStream = "stdout"
	LogStderr LogStream = "stderr"
)

type logTickMsg struct{}

type logStreamState struct {
	path      string
	offset    int64
	carry     string
	lines     []string
	lastErr   string
	seenFirst bool
}

type ServiceModel struct {
	width  int
	height int

	last *tui.StateSnapshot

	name string

	active LogStream
	follow bool

	searching bool
	search    textinput.Model
	filter    string

	tailLines int
	maxLines  int
	tickEvery time.Duration

	stdout logStreamState
	stderr logStreamState

	vp viewport.Model
}

func NewServiceModel() ServiceModel {
	search := textinput.New()
	search.Placeholder = "filter…"
	search.Prompt = "/ "
	search.CharLimit = 200

	m := ServiceModel{
		active:    LogStdout,
		follow:    true,
		search:    search,
		tailLines: 200,
		maxLines:  2000,
		tickEvery: 250 * time.Millisecond,
	}
	m.vp = viewport.New(0, 0)
	return m
}

func (m ServiceModel) WithSize(width, height int) ServiceModel {
	m.width, m.height = width, height
	m = m.resizeViewport()
	return m
}

func (m ServiceModel) WithSnapshot(s tui.StateSnapshot) ServiceModel {
	m.last = &s
	m = m.syncPathsFromSnapshot()
	return m
}

func (m ServiceModel) WithService(name string) ServiceModel {
	m.name = name
	m.active = LogStdout
	m.follow = true
	m.searching = false
	m.filter = ""
	m.search.SetValue("")
	m.search.Blur()
	m.stdout = logStreamState{}
	m.stderr = logStreamState{}
	m = m.syncPathsFromSnapshot()
	m = m.loadInitialTail()
	return m
}

func (m ServiceModel) Update(msg tea.Msg) (ServiceModel, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		w, h := v.Width, v.Height
		if w <= 0 {
			w = 80
		}
		if h <= 0 {
			h = 24
		}
		m.width, m.height = w, h
		m = m.resizeViewport()
		return m, nil
	case tea.KeyMsg:
		if m.searching {
			switch v.String() {
			case "esc":
				m.searching = false
				m.search.Blur()
				return m, nil
			case "enter":
				m.filter = strings.TrimSpace(m.search.Value())
				m.searching = false
				m.search.Blur()
				m = m.refreshViewportContent(true)
				return m, nil
			}

			var cmd tea.Cmd
			m.search, cmd = m.search.Update(v)
			return m, cmd
		}

		switch v.String() {
		case "/":
			m.searching = true
			m.search.SetValue(m.filter)
			m.search.CursorEnd()
			m.search.Focus()
			return m, nil
		case "ctrl+l":
			m.filter = ""
			m.search.SetValue("")
			m = m.refreshViewportContent(true)
			return m, nil
		case "tab":
			if m.active == LogStdout {
				m.active = LogStderr
			} else {
				m.active = LogStdout
			}
			m = m.refreshViewportContent(true)
			return m, nil
		case "f":
			m.follow = !m.follow
			if m.follow {
				return m, m.tickCmd()
			}
			return m, nil
		}

		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(v)
		if cmd != nil {
			return m, cmd
		}
		return m, nil
	case logTickMsg:
		if m.name == "" || !m.follow {
			return m, nil
		}
		m = m.tickReadAll()
		m = m.refreshViewportContent(true)
		return m, m.tickCmd()
	}
	return m, nil
}

func (m ServiceModel) View() string {
	if m.name == "" {
		return "No service selected.\n"
	}

	rec, alive, found := m.lookupService()
	if !found {
		return fmt.Sprintf("Service: %s\n\nNo record for this service in the current state snapshot.\n", m.name)
	}

	status := "dead"
	if alive {
		status = "alive"
	}
	streamLabel := string(m.active)
	followLabel := "off"
	if m.follow {
		followLabel = "on"
	}
	filterLabel := ""
	if m.filter != "" {
		filterLabel = fmt.Sprintf(" filter=%q", m.filter)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Service: %s  (%s)  pid=%d  %s  follow=%s%s\n", m.name, status, rec.PID, streamLabel, followLabel, filterLabel))
	b.WriteString("tab switch stream, f follow, / filter, ctrl+l clear, esc back\n")
	b.WriteString("\n")

	if m.activeState().path != "" {
		b.WriteString(fmt.Sprintf("Path: %s\n\n", m.activeState().path))
	} else {
		b.WriteString("Path: (unknown)\n\n")
	}

	if errText := m.activeState().lastErr; errText != "" {
		b.WriteString(fmt.Sprintf("log error: %s\n\n", errText))
	}

	if m.searching {
		b.WriteString(m.search.View())
		b.WriteString("\n\n")
	}

	b.WriteString(m.vp.View())
	return b.String()
}

func (m ServiceModel) tickCmd() tea.Cmd {
	if m.tickEvery <= 0 {
		m.tickEvery = 250 * time.Millisecond
	}
	return tea.Tick(m.tickEvery, func(time.Time) tea.Msg { return logTickMsg{} })
}

func (m ServiceModel) resizeViewport() ServiceModel {
	usableHeight := m.height - 6
	if usableHeight < 3 {
		usableHeight = 3
	}
	m.vp.Width = maxInt(0, m.width)
	m.vp.Height = usableHeight
	m.vp.HighPerformanceRendering = false
	m = m.refreshViewportContent(false)
	return m
}

func (m ServiceModel) activeState() *logStreamState {
	if m.active == LogStderr {
		return &m.stderr
	}
	return &m.stdout
}

func (m ServiceModel) lookupService() (*state.ServiceRecord, bool, bool) {
	if m.last == nil || m.last.State == nil {
		return nil, false, false
	}
	for i := range m.last.State.Services {
		svc := &m.last.State.Services[i]
		if svc.Name == m.name {
			alive := false
			if m.last.Alive != nil {
				alive = m.last.Alive[svc.Name]
			}
			return svc, alive, true
		}
	}
	return nil, false, false
}

func (m ServiceModel) syncPathsFromSnapshot() ServiceModel {
	rec, _, found := m.lookupService()
	if !found || rec == nil {
		return m
	}
	m.stdout.path = rec.StdoutLog
	m.stderr.path = rec.StderrLog
	return m
}

func (m ServiceModel) loadInitialTail() ServiceModel {
	m.stdout = m.loadTailForStream(m.stdout)
	m.stderr = m.loadTailForStream(m.stderr)
	m = m.refreshViewportContent(true)
	return m
}

func (m ServiceModel) loadTailForStream(s logStreamState) logStreamState {
	s.lastErr = ""
	s.lines = nil
	s.carry = ""
	s.offset = 0
	s.seenFirst = true

	if s.path == "" {
		s.lastErr = "missing log path"
		return s
	}

	lines, offset, err := readTailLines(s.path, m.tailLines, 2<<20)
	if err != nil {
		s.lastErr = err.Error()
		return s
	}
	s.lines = lines
	s.offset = offset
	return s
}

func (m ServiceModel) tickReadAll() ServiceModel {
	m.stdout = m.readNewBytes(m.stdout)
	m.stderr = m.readNewBytes(m.stderr)
	return m
}

func (m ServiceModel) readNewBytes(s logStreamState) logStreamState {
	s.lastErr = ""
	if s.path == "" {
		s.lastErr = "missing log path"
		return s
	}

	f, err := os.Open(s.path)
	if err != nil {
		s.lastErr = err.Error()
		return s
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		s.lastErr = err.Error()
		return s
	}
	size := info.Size()
	if size < s.offset {
		s.offset = 0
		s.lines = nil
		s.carry = ""
	}

	if _, err := f.Seek(s.offset, io.SeekStart); err != nil {
		s.lastErr = err.Error()
		return s
	}

	const maxRead = 256 << 10
	buf, err := io.ReadAll(io.LimitReader(f, maxRead))
	if err != nil {
		s.lastErr = err.Error()
		return s
	}
	if len(buf) == 0 {
		return s
	}
	s.offset += int64(len(buf))

	text := s.carry + string(buf)
	parts := strings.Split(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		s.carry = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	} else {
		s.carry = ""
		if len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
	}
	for _, line := range parts {
		s.lines = append(s.lines, line)
	}
	if m.maxLines > 0 && len(s.lines) > m.maxLines {
		s.lines = append([]string{}, s.lines[len(s.lines)-m.maxLines:]...)
	}
	return s
}

func (m ServiceModel) refreshViewportContent(gotoBottom bool) ServiceModel {
	s := m.activeState()
	content := ""
	if len(s.lines) == 0 {
		content = "(no log lines yet)\n"
	} else {
		lines := s.lines
		if m.filter != "" {
			filtered := make([]string, 0, len(lines))
			for _, line := range lines {
				if strings.Contains(line, m.filter) {
					filtered = append(filtered, line)
				}
			}
			lines = filtered
		}
		if len(lines) == 0 {
			content = "(no matching lines)\n"
		} else {
			content = strings.Join(lines, "\n") + "\n"
		}
	}
	m.vp.SetContent(content)
	if gotoBottom && m.follow {
		m.vp.GotoBottom()
	}
	return m
}

func readTailLines(path string, tailLines int, maxBytes int64) ([]string, int64, error) {
	if tailLines <= 0 {
		tailLines = 200
	}
	if maxBytes <= 0 {
		maxBytes = 2 << 20
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	size := info.Size()
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, 0, err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, 0, err
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

	return lines, size, nil
}
