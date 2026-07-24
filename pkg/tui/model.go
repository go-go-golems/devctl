package tui

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/go-go-golems/devctl/pkg/runlog"
)

type Options struct {
	Context    context.Context
	Controller operator.Controller
	RepoRoot   string
	ConfigPath string
	Profile    string
	Cwd        string
	Strict     bool
	DryRun     bool
	Timeout    time.Duration
	Refresh    time.Duration
}

type confirmation struct {
	Kind     string
	Services []string
}

type Model struct {
	options      Options
	active       ViewKind
	overview     OverviewModel
	logs         LogsModel
	runs         RunsModel
	width        int
	height       int
	status       string
	confirmation *confirmation
	palette      bool
	help         bool
	searching    bool
	cursors      map[string]runlog.Cursor
	runIDs       []string
	following    bool
	operationCh  <-chan tea.Msg
}

var _ tea.Model = (*Model)(nil)

func NewModel(options Options) *Model {
	if options.Context == nil {
		options.Context = context.Background()
	}
	if options.Refresh <= 0 {
		options.Refresh = time.Second
	}
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Second
	}
	return &Model{
		options: options, active: ViewOverview, logs: NewLogsModel(),
		cursors: map[string]runlog.Cursor{},
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.snapshotCmd(), m.snapshotTickCmd())
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		return m, nil
	case snapshotTickMsg:
		return m, tea.Batch(m.snapshotCmd(), m.snapshotTickCmd())
	case SnapshotMsg:
		if message.Snapshot.Revision == m.overview.Snapshot.Revision &&
			message.Snapshot.Exists == m.overview.Snapshot.Exists {
			m.overview.SetSnapshot(message.Snapshot)
			m.updateLogRuns(message.Snapshot)
			return m, nil
		}
		m.overview.SetSnapshot(message.Snapshot)
		m.updateLogRuns(message.Snapshot)
		if m.active == ViewLogs && m.logs.Follow {
			return m, m.followOneCmd()
		}
		return m, nil
	case LogMsg:
		m.following = false
		m.logs.Add(message.Record)
		m.cursors[message.Record.RunID] = runlog.Cursor{
			RunID: message.Record.RunID, Sequence: message.Record.Sequence,
		}
		if m.active == ViewLogs && m.logs.Follow {
			return m, m.followOneCmd()
		}
		return m, nil
	case followStoppedMsg:
		m.following = false
		if message.Err != nil && !stderrors.Is(message.Err, context.Canceled) {
			m.status = fmt.Sprintf("follow logs: %v", message.Err)
		}
		return m, nil
	case EventMsg:
		m.status = eventStatus(message.Event)
		return m, waitOperationMsg(m.operationCh)
	case OperationDoneMsg:
		m.runs.Add(message.Result, message.Events, message.Err)
		m.status = operationStatus(message)
		m.confirmation = nil
		m.operationCh = nil
		return m, m.snapshotCmd()
	case ErrorMsg:
		m.status = fmt.Sprintf("%s: %v", message.Operation, message.Err)
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(message)
	}
	return m, nil
}

func (m *Model) updateKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	value := key.String()
	if m.searching {
		switch value {
		case "esc":
			m.searching = false
		case "enter":
			m.searching = false
		case "backspace":
			if len(m.logs.Search) > 0 {
				m.logs.Search = m.logs.Search[:len(m.logs.Search)-1]
			}
		default:
			if len(key.Runes) > 0 {
				m.logs.Search += string(key.Runes)
			}
		}
		return m, nil
	}
	if m.confirmation != nil {
		switch value {
		case "y", "enter":
			return m, m.operationCmd(*m.confirmation)
		case "n", "esc", "q":
			m.confirmation = nil
		}
		return m, nil
	}
	if m.palette {
		if value == "esc" || value == ":" || value == "q" {
			m.palette = false
		}
		return m, nil
	}
	switch value {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "1":
		m.active = ViewOverview
	case "2":
		m.active = ViewLogs
		if m.logs.Follow {
			return m, m.followOneCmd()
		}
	case "3":
		m.active = ViewRuns
	case "tab":
		m.active = (m.active + 1) % 3
		if m.active == ViewLogs && m.logs.Follow {
			return m, m.followOneCmd()
		}
	case "esc":
		m.active = ViewOverview
	case "?":
		m.help = !m.help
	case ":":
		m.palette = true
	case "j", "down":
		m.moveSelection(1)
	case "k", "up":
		m.moveSelection(-1)
	case "enter":
		if m.active == ViewOverview {
			m.logs.Services = m.overview.SelectedServices()
			m.active = ViewLogs
			return m, m.followOneCmd()
		}
	case "u", "d", "r":
		if m.active == ViewOverview {
			kind := map[string]string{"u": "up", "d": "down", "r": "restart"}[value]
			m.confirmation = &confirmation{Kind: kind, Services: m.overview.SelectedServices()}
		}
	case "p":
		if m.active == ViewLogs {
			m.logs.TogglePause()
		}
	case "f":
		if m.active == ViewLogs {
			m.logs.Follow = !m.logs.Follow
			if m.logs.Follow {
				return m, m.followOneCmd()
			}
		}
	case "w":
		if m.active == ViewLogs {
			m.logs.Wrap = !m.logs.Wrap
		}
	case "/":
		if m.active == ViewLogs {
			m.searching = true
		}
	case "l":
		if m.active == ViewRuns {
			m.active = ViewLogs
			return m, m.followOneCmd()
		}
	}
	return m, nil
}

func (m *Model) View() string {
	width := max(40, m.width)
	height := max(12, m.height)
	profile := emptyDash(m.overview.Snapshot.Profile)
	environment := "stopped"
	if m.overview.Snapshot.Exists {
		environment = "known"
	}
	header := fmt.Sprintf(
		"devctl  profile=%s  environment=%s  revision=%d\n%s\n",
		profile, environment, m.overview.Snapshot.Revision, m.navigation(),
	)
	var body string
	switch m.active {
	case ViewOverview:
		body = m.overview.View(width)
	case ViewLogs:
		body = m.logs.View(height - 6)
	case ViewRuns:
		body = m.runs.View(height - 6)
	default:
		body = "Unknown view"
	}
	footer := "\n\n" + truncate(m.status, width)
	if m.confirmation != nil {
		footer += fmt.Sprintf(
			"\nConfirm %s for services [%s]? [y] yes [n] no",
			m.confirmation.Kind, strings.Join(m.confirmation.Services, ", "),
		)
	}
	if m.palette {
		footer += "\nCommands: refresh snapshot | doctor | plugin inspection: devctl plugins inspect"
	}
	if m.searching {
		footer += "\nSearch: " + m.logs.Search + "█"
	}
	if m.help {
		footer += "\nKeys: 1/2/3 views, Tab cycle, Esc overview, : commands, ? help, q quit"
	}
	return header + strings.Repeat("─", min(width, 100)) + "\n" + body + footer
}

func (m *Model) moveSelection(delta int) {
	switch m.active {
	case ViewOverview:
		m.overview.Move(delta)
	case ViewRuns:
		m.runs.Move(delta)
	case ViewLogs:
	}
}

func (m *Model) navigation() string {
	labels := []string{"[1] Overview", "[2] Logs", "[3] Runs"}
	labels[m.active] = "> " + labels[m.active] + " <"
	return strings.Join(labels, "   ") + "   [:] Commands"
}

func (m *Model) snapshotCmd() tea.Cmd {
	return func() tea.Msg {
		snapshot, err := m.options.Controller.Snapshot(m.options.Context, operator.SnapshotRequest{
			RepoRoot: m.options.RepoRoot, IncludeRuns: true, IncludeHealth: true,
		})
		if err != nil {
			return ErrorMsg{Operation: "snapshot", Err: err}
		}
		return SnapshotMsg{Snapshot: snapshot}
	}
}

func (m *Model) snapshotTickCmd() tea.Cmd {
	return tea.Tick(m.options.Refresh, func(time.Time) tea.Msg { return snapshotTickMsg{} })
}

func (m *Model) operationCmd(request confirmation) tea.Cmd {
	messages := make(chan tea.Msg)
	m.operationCh = messages
	go func() {
		defer close(messages)
		sink := &channelEventSink{ctx: m.options.Context, messages: messages}
		selection := operator.Selection{Services: append([]string{}, request.Services...)}
		policy := operator.PipelinePolicy{
			ConfigPath: m.options.ConfigPath, Cwd: m.options.Cwd,
			Strict: m.options.Strict, DryRun: m.options.DryRun, Timeout: m.options.Timeout,
		}
		var result operator.OperationResult
		var err error
		switch request.Kind {
		case "up":
			result, err = m.options.Controller.Up(m.options.Context, operator.UpRequest{
				RepoRoot: m.options.RepoRoot, Profile: m.options.Profile,
				Select: selection, Policy: policy,
			}, sink)
		case "down":
			result, err = m.options.Controller.Down(m.options.Context, operator.DownRequest{
				RepoRoot: m.options.RepoRoot, Select: selection,
			}, sink)
		case "restart":
			result, err = m.options.Controller.Restart(m.options.Context, operator.RestartRequest{
				RepoRoot: m.options.RepoRoot, Profile: m.options.Profile,
				Select: selection, Policy: policy,
			}, sink)
		}
		select {
		case messages <- OperationDoneMsg{Result: result, Events: sink.events, Err: err}:
		case <-m.options.Context.Done():
		}
	}()
	return waitOperationMsg(messages)
}

func (m *Model) updateLogRuns(snapshot operator.Snapshot) {
	runIDs := make([]string, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		if service.RunID != "" {
			runIDs = append(runIDs, service.RunID)
		}
	}
	m.runIDs = runIDs
}

var errOneRecord = stderrors.New("one log record delivered")

func (m *Model) followOneCmd() tea.Cmd {
	reader := m.options.Controller.Logs()
	if reader == nil || len(m.runIDs) == 0 || !m.logs.Follow || m.following {
		return nil
	}
	m.following = true
	request := runlog.FollowRequest{
		Query: runlog.Query{
			RunIDs: append([]string{}, m.runIDs...), Services: append([]string{}, m.logs.Services...),
			Streams: append([]runlog.StreamKind{}, m.logs.Streams...),
		},
		After: cloneCursors(m.cursors),
	}
	return func() tea.Msg {
		sink := &oneRecordSink{}
		err := reader.Follow(m.options.Context, request, sink)
		if stderrors.Is(err, errOneRecord) {
			return LogMsg{Record: sink.record}
		}
		if err != nil && !stderrors.Is(err, context.Canceled) {
			return followStoppedMsg{Err: err}
		}
		return followStoppedMsg{Err: err}
	}
}

type channelEventSink struct {
	ctx      context.Context
	messages chan<- tea.Msg
	events   []operator.OperatorEvent
}

func (s *channelEventSink) Send(ctx context.Context, event operator.OperatorEvent) error {
	s.events = append(s.events, event)
	select {
	case s.messages <- EventMsg{Event: event}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

type oneRecordSink struct {
	record runlog.LogRecord
}

func (s *oneRecordSink) Add(_ context.Context, record runlog.LogRecord) error {
	s.record = record
	return errOneRecord
}

func cloneCursors(input map[string]runlog.Cursor) map[string]runlog.Cursor {
	output := make(map[string]runlog.Cursor, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func waitOperationMsg(messages <-chan tea.Msg) tea.Cmd {
	if messages == nil {
		return nil
	}
	return func() tea.Msg {
		message, ok := <-messages
		if !ok {
			return nil
		}
		return message
	}
}

func eventStatus(event operator.OperatorEvent) string {
	if event.Error != nil {
		return fmt.Sprintf("%s: %s", event.Error.Code, event.Error.Message)
	}
	return fmt.Sprintf("%s: %s", event.Kind, event.Status)
}

func operationStatus(message OperationDoneMsg) string {
	if message.Err != nil {
		return fmt.Sprintf("%s %s: %v", message.Result.Kind, message.Result.Status, message.Err)
	}
	return fmt.Sprintf("%s %s", message.Result.Kind, message.Result.Status)
}
