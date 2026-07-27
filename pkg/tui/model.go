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

type paletteAction struct {
	Label string
	Kind  string
}

var paletteActions = []paletteAction{
	{Label: "Refresh environment snapshot", Kind: "refresh"},
	{Label: "Run operator diagnostics", Kind: "doctor"},
	{Label: "Go to Overview", Kind: "overview"},
	{Label: "Go to Logs", Kind: "logs"},
	{Label: "Go to Runs", Kind: "runs"},
	{Label: "Show logs for selected service", Kind: "selected-logs"},
	{Label: "Toggle log follow", Kind: "toggle-follow"},
	{Label: "Toggle log pause", Kind: "toggle-pause"},
	{Label: "Clear local log buffer", Kind: "clear-logs"},
	{Label: "Show plugin inspection command", Kind: "plugins"},
}

type activeOperation struct {
	ID        string
	Kind      string
	StartedAt time.Time
	LastEvent operator.OperatorEvent
	Recent    []operator.OperatorEvent
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
	paletteIndex int
	help         bool
	searching    bool
	cursors      map[string]runlog.Cursor
	runIDs       []string
	following    bool
	operationCh  <-chan tea.Msg
	operation    *activeOperation
	now          func() time.Time
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
		cursors: map[string]runlog.Cursor{}, now: time.Now,
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
		m.recordOperationEvent(message.Event)
		return m, waitOperationMsg(m.operationCh)
	case OperationDoneMsg:
		m.runs.Add(message.Result, message.Events, message.Err)
		m.status = operationStatus(message)
		m.confirmation = nil
		m.operationCh = nil
		if m.operation != nil {
			m.operation.LastEvent.Status = message.Result.Status
		}
		return m, m.snapshotCmd()
	case DoctorMsg:
		if message.Err != nil {
			m.status = fmt.Sprintf("doctor: %v", message.Err)
			return m, nil
		}
		failed := 0
		for _, check := range message.Report.Checks {
			if check.Status != "ok" {
				failed++
			}
		}
		m.status = fmt.Sprintf("doctor: %d checks, %d require attention", len(message.Report.Checks), failed)
		return m, nil
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
			request := *m.confirmation
			m.confirmation = nil
			if m.operationCh != nil {
				return m, nil
			}
			return m, m.operationCmd(request)
		case "n", "esc", "q":
			m.confirmation = nil
		}
		return m, nil
	}
	if m.palette {
		switch value {
		case "esc", ":", "q":
			m.palette = false
		case "j", "down":
			m.paletteIndex = (m.paletteIndex + 1) % len(paletteActions)
		case "k", "up":
			m.paletteIndex = (m.paletteIndex - 1 + len(paletteActions)) % len(paletteActions)
		case "enter":
			action := paletteActions[m.paletteIndex]
			m.palette = false
			switch action.Kind {
			case "refresh":
				m.status = "refreshing environment snapshot"
				return m, m.snapshotCmd()
			case "doctor":
				m.status = "running operator diagnostics"
				return m, m.doctorCmd()
			case "plugins":
				m.status = "inspect plugin providers with: devctl plugins inspect"
			case "overview":
				m.active = ViewOverview
			case "logs":
				m.active = ViewLogs
			case "runs":
				m.active = ViewRuns
			case "selected-logs":
				m.logs.Services = m.overview.SelectedServices()
				m.active = ViewLogs
				return m, m.followOneCmd()
			case "toggle-follow":
				m.logs.Follow = !m.logs.Follow
				if m.logs.Follow {
					return m, m.followOneCmd()
				}
			case "toggle-pause":
				m.logs.TogglePause()
			case "clear-logs":
				m.logs.Clear()
			}
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
		if m.active == ViewOverview && m.operationCh == nil {
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
	case "o":
		if m.active == ViewLogs {
			m.logs.ToggleStream(runlog.StreamStdout)
		}
	case "e":
		if m.active == ViewLogs {
			m.logs.ToggleStream(runlog.StreamStderr)
		}
	case "x":
		if m.active == ViewLogs {
			m.logs.Clear()
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
	running, unhealthy := snapshotCounts(m.overview.Snapshot)
	header := titleStyle.Render("devctl") + "  " +
		mutedStyle.Render("profile") + "=" + accentStyle.Render(profile) + "  " +
		mutedStyle.Render("environment") + "=" + stateStyle(environment).Render(environment) + "  " +
		summaryBadge("services", len(m.overview.Snapshot.Services)) + "  " +
		summaryBadge("running", running) + "  " + summaryBadge("unhealthy", unhealthy) + "\n" +
		m.navigation() + "\n"
	var body string
	switch m.active {
	case ViewOverview:
		body = m.overview.ViewAt(width, m.now())
	case ViewLogs:
		body = m.logs.View(height - 6)
	case ViewRuns:
		body = m.runs.View(height - 6)
	default:
		body = "Unknown view"
	}
	footer := "\n" + m.operationView(width) + "\n" + clipLine(m.status, width)
	if m.confirmation != nil {
		targets := strings.Join(m.confirmation.Services, ", ")
		if targets == "" {
			targets = "all configured services"
		}
		footer += fmt.Sprintf(
			"\nConfirm %s for services [%s]? [y] yes [n] no",
			m.confirmation.Kind, targets,
		)
	}
	if m.palette {
		footer += "\n" + titleStyle.Render("Commands") + " " + mutedStyle.Render("[j/k select, enter run, esc close]")
		for index, action := range paletteActions {
			cursor := " "
			if index == m.paletteIndex {
				cursor = ">"
			}
			footer += fmt.Sprintf("\n%s %s", cursor, action.Label)
		}
	}
	if m.searching {
		footer += "\nSearch: " + m.logs.Search + "█"
	}
	if m.help {
		footer += "\nKeys: 1/2/3 views, Tab cycle, Esc overview, : commands, ? help, q quit"
	}
	return header + borderStyle.Render(strings.Repeat("─", width)) + "\n" + body + footer
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
	for index := range labels {
		if index == int(m.active) {
			labels[index] = activeStyle.Render(labels[index])
		} else {
			labels[index] = mutedStyle.Render(labels[index])
		}
	}
	return strings.Join(labels, "   ") + "   " + renderKey(":", "Commands")
}

func snapshotCounts(snapshot operator.Snapshot) (int, int) {
	running, unhealthy := 0, 0
	for _, service := range snapshot.Services {
		if service.Phase == "ready" || service.Phase == "starting" {
			running++
		}
		if service.Health != nil && !service.Health.Healthy {
			unhealthy++
		}
	}
	return running, unhealthy
}

func (m *Model) recordOperationEvent(event operator.OperatorEvent) {
	if m.operation == nil || (event.OperationID != "" && event.OperationID != m.operation.ID) {
		m.operation = &activeOperation{ID: event.OperationID, Kind: string(event.Kind), StartedAt: event.At}
	}
	if m.operation.StartedAt.IsZero() {
		m.operation.StartedAt = m.now()
	}
	m.operation.LastEvent = event
	m.operation.Recent = append(m.operation.Recent, event)
	if len(m.operation.Recent) > 8 {
		m.operation.Recent = m.operation.Recent[len(m.operation.Recent)-8:]
	}
}

func (m *Model) operationView(width int) string {
	if m.operation == nil {
		return mutedStyle.Render("No active operation")
	}
	event := m.operation.LastEvent
	label := event.Phase
	if event.Service != "" {
		label = event.Service
	}
	if label == "" {
		label = string(event.Kind)
	}
	elapsed := shortAge(m.now().Sub(m.operation.StartedAt))
	line := fmt.Sprintf("Operation %s  %-18s %-10s elapsed %s", emptyDash(m.operation.ID), label, event.Status, elapsed)
	return clipLine(stateStyle(event.Status).Render(line), width)
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

func (m *Model) doctorCmd() tea.Cmd {
	return func() tea.Msg {
		report, err := m.options.Controller.Doctor(m.options.Context, operator.DoctorRequest{
			RepoRoot: m.options.RepoRoot, ConfigPath: m.options.ConfigPath,
			Profile: m.options.Profile, Cwd: m.options.Cwd, Timeout: m.options.Timeout,
			Plugins: true,
		})
		return DoctorMsg{Report: report, Err: err}
	}
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
				RepoRoot: m.options.RepoRoot, Select: selection, Timeout: m.options.Timeout,
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
	m.runs.SetSnapshot(snapshot)
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
