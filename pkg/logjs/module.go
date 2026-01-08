package logjs

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

var ErrNoRegister = errors.New("logjs: script did not call register()")
var ErrHookTimeout = errors.New("logjs: js hook timeout")

type Module struct {
	vm     *goja.Runtime
	opts   options
	config *goja.Object

	scriptPath string

	name string
	tag  string

	parseFn     goja.Callable
	filterFn    goja.Callable
	transformFn goja.Callable
	initFn      goja.Callable
	shutdownFn  goja.Callable
	onErrorFn   goja.Callable

	state *goja.Object
	stats Stats
}

type options struct {
	hookTimeout time.Duration
}

func ParseOptions(opts Options) (options, error) {
	var out options
	if opts.HookTimeout != "" {
		d, err := time.ParseDuration(opts.HookTimeout)
		if err != nil {
			return options{}, errors.Wrap(err, "parse --js-timeout")
		}
		out.hookTimeout = d
	}
	return out, nil
}

func LoadFromFile(ctx context.Context, scriptPath string, opts Options) (*Module, error) {
	_ = ctx

	parsedOpts, err := ParseOptions(opts)
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, errors.Wrap(err, "read script")
	}

	m := &Module{
		vm:         goja.New(),
		opts:       parsedOpts,
		scriptPath: scriptPath,
		state:      nil,
	}

	enableConsole(m.vm)

	m.state = m.vm.NewObject()

	if err := m.vm.Set("register", func(config goja.Value) error {
		if m.config != nil {
			return errors.New("register() called more than once")
		}
		if goja.IsNull(config) || goja.IsUndefined(config) {
			return errors.New("register(config) requires a config object")
		}
		m.config = config.ToObject(m.vm)
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "set register")
	}

	if _, err := m.vm.RunScript("logjs:helpers", helpersJS); err != nil {
		return nil, errors.Wrap(err, "load helpers")
	}

	if err := injectGoHelpers(m); err != nil {
		return nil, err
	}

	prog, err := goja.Compile(scriptPath, string(b), false)
	if err != nil {
		return nil, errors.Wrap(err, "compile script")
	}
	if _, err := m.vm.RunProgram(prog); err != nil {
		return nil, errors.Wrap(err, "run script")
	}

	if m.config == nil {
		return nil, ErrNoRegister
	}

	nameVal := m.config.Get("name")
	if isNullish(nameVal) || strings.TrimSpace(nameVal.String()) == "" {
		return nil, errors.New("register({ name: string, ... }): name is required")
	}
	m.name = nameVal.String()
	m.tag = m.name

	tagVal := m.config.Get("tag")
	if !isNullish(tagVal) && strings.TrimSpace(tagVal.String()) != "" {
		m.tag = tagVal.String()
	}

	parseVal := m.config.Get("parse")
	parseFn, ok := goja.AssertFunction(parseVal)
	if !ok {
		return nil, errors.New("register({ parse: function(line, ctx), ... }): parse is required")
	}
	m.parseFn = parseFn

	if fn, ok := goja.AssertFunction(m.config.Get("filter")); ok {
		m.filterFn = fn
	}
	if fn, ok := goja.AssertFunction(m.config.Get("transform")); ok {
		m.transformFn = fn
	}
	if fn, ok := goja.AssertFunction(m.config.Get("init")); ok {
		m.initFn = fn
	}
	if fn, ok := goja.AssertFunction(m.config.Get("shutdown")); ok {
		m.shutdownFn = fn
	}
	if fn, ok := goja.AssertFunction(m.config.Get("onError")); ok {
		m.onErrorFn = fn
	}

	if m.initFn != nil {
		ctxObj := m.buildContext("init", "init", 0)
		if _, err := m.callHook("init", m.initFn, ctxObj); err != nil {
			m.stats.HookErrors++
			m.callOnError("init", err, goja.Undefined(), ctxObj)
		}
	}

	return m, nil
}

func (m *Module) Name() string {
	return m.name
}

func (m *Module) Tag() string {
	if strings.TrimSpace(m.tag) == "" {
		return m.name
	}
	return m.tag
}

func (m *Module) ScriptPath() string {
	return m.scriptPath
}

func (m *Module) Stats() Stats {
	return m.stats
}

func (m *Module) Info() ModuleInfo {
	return ModuleInfo{
		Name:         m.Name(),
		Tag:          m.Tag(),
		HasParse:     m.parseFn != nil,
		HasFilter:    m.filterFn != nil,
		HasTransform: m.transformFn != nil,
		HasInit:      m.initFn != nil,
		HasShutdown:  m.shutdownFn != nil,
		HasOnError:   m.onErrorFn != nil,
	}
}

func (m *Module) Close(ctx context.Context) error {
	_ = ctx

	if m.shutdownFn == nil {
		return nil
	}

	ctxObj := m.buildContext("shutdown", "shutdown", 0)
	if _, err := m.callHook("shutdown", m.shutdownFn, ctxObj); err != nil {
		m.stats.HookErrors++
		m.callOnError("shutdown", err, goja.Undefined(), ctxObj)
	}
	return nil
}

func (m *Module) ProcessLine(ctx context.Context, line string, source string, lineNumber int64) ([]*Event, []*ErrorRecord, error) {
	_ = ctx

	m.stats.LinesProcessed++

	trimmed := trimTrailingNewline(line)
	ctxObj := m.buildContext("parse", source, lineNumber)

	v, err := m.callHook("parse", m.parseFn, m.vm.ToValue(trimmed), ctxObj)
	if err != nil {
		m.stats.HookErrors++
		m.callOnError("parse", err, m.vm.ToValue(trimmed), ctxObj)
		m.stats.LinesDropped++
		return nil, []*ErrorRecord{m.newErrorRecord("parse", err, source, lineNumber, &trimmed)}, nil
	}

	eventLikes, drop, err := m.ensureEventLikes(v)
	if err != nil {
		m.stats.HookErrors++
		m.callOnError("parse", err, m.vm.ToValue(trimmed), ctxObj)
		m.stats.LinesDropped++
		return nil, []*ErrorRecord{m.newErrorRecord("parse", err, source, lineNumber, &trimmed)}, nil
	}
	if drop {
		m.stats.LinesDropped++
		return nil, nil, nil
	}

	outEvents := make([]*Event, 0, len(eventLikes))
	outErrors := make([]*ErrorRecord, 0)

	for _, eventLike := range eventLikes {
		if m.filterFn != nil {
			ctxObj = m.buildContext("filter", source, lineNumber)
			keepVal, err := m.callHook("filter", m.filterFn, eventLike, ctxObj)
			if err != nil {
				m.stats.HookErrors++
				m.callOnError("filter", err, eventLike, ctxObj)
				m.stats.LinesDropped++
				outErrors = append(outErrors, m.newErrorRecord("filter", err, source, lineNumber, &trimmed))
				continue
			}
			if !keepVal.ToBoolean() {
				m.stats.LinesDropped++
				continue
			}
		}

		eventLikesAfterTransform := []goja.Value{eventLike}
		if m.transformFn != nil {
			ctxObj = m.buildContext("transform", source, lineNumber)
			outVal, err := m.callHook("transform", m.transformFn, eventLike, ctxObj)
			if err != nil {
				m.stats.HookErrors++
				m.callOnError("transform", err, eventLike, ctxObj)
				m.stats.LinesDropped++
				outErrors = append(outErrors, m.newErrorRecord("transform", err, source, lineNumber, &trimmed))
				continue
			}

			outs, drop, err := m.ensureEventLikes(outVal)
			if err != nil {
				m.stats.HookErrors++
				m.callOnError("transform", err, outVal, ctxObj)
				m.stats.LinesDropped++
				outErrors = append(outErrors, m.newErrorRecord("transform", err, source, lineNumber, &trimmed))
				continue
			}
			if drop {
				m.stats.LinesDropped++
				continue
			}
			eventLikesAfterTransform = outs
		}

		for _, evLike := range eventLikesAfterTransform {
			ev, err := m.normalizeEvent(evLike, source, trimmed, lineNumber)
			if err != nil {
				m.stats.HookErrors++
				ctxObj = m.buildContext("transform", source, lineNumber)
				m.callOnError("transform", err, evLike, ctxObj)
				m.stats.LinesDropped++
				outErrors = append(outErrors, m.newErrorRecord("transform", err, source, lineNumber, &trimmed))
				continue
			}

			m.stats.EventsEmitted++
			outEvents = append(outEvents, ev)
		}
	}

	return outEvents, outErrors, nil
}

func trimTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		s = strings.TrimSuffix(s, "\n")
		s = strings.TrimSuffix(s, "\r")
	}
	return s
}

func (m *Module) buildContext(hook string, source string, lineNumber int64) *goja.Object {
	obj := m.vm.NewObject()

	_ = obj.Set("hook", hook)
	_ = obj.Set("source", source)
	_ = obj.Set("lineNumber", lineNumber)
	_ = obj.Set("state", m.state)

	now := time.Now().UTC()
	_ = obj.Set("now", m.newDate(now))

	return obj
}

func (m *Module) newDate(t time.Time) goja.Value {
	ctor := m.vm.Get("Date")
	o, err := m.vm.New(ctor, m.vm.ToValue(t.UnixMilli()))
	if err != nil {
		return goja.Undefined()
	}
	return o
}

func (m *Module) callHook(hook string, fn goja.Callable, args ...goja.Value) (goja.Value, error) {
	if fn == nil {
		return goja.Undefined(), nil
	}

	timeout := m.opts.hookTimeout
	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			m.vm.Interrupt(ErrHookTimeout)
		})
		defer timer.Stop()
		defer m.vm.ClearInterrupt()
	}

	v, err := fn(goja.Undefined(), args...)
	if err != nil {
		if isInterruptedByTimeout(err) {
			m.stats.HookTimeouts++
		}
		return nil, err
	}
	return v, nil
}

func (m *Module) callOnError(hook string, err error, payload goja.Value, ctxObj *goja.Object) {
	if m.onErrorFn == nil {
		return
	}

	_ = ctxObj.Set("hook", hook)

	_, _ = m.onErrorFn(goja.Undefined(), m.vm.ToValue(err), payload, ctxObj)
}

func (m *Module) ensureEventLike(v goja.Value) (goja.Value, bool, error) {
	if v == nil {
		return goja.Undefined(), true, nil
	}
	if goja.IsNull(v) || goja.IsUndefined(v) {
		return goja.Undefined(), true, nil
	}
	if s, ok := v.Export().(string); ok {
		obj := m.vm.NewObject()
		_ = obj.Set("message", s)
		return obj, false, nil
	}
	if _, ok := v.(*goja.Object); !ok {
		return goja.Undefined(), false, errors.Errorf("event must be an object or string, got %T", v.Export())
	}
	return v, false, nil
}

func (m *Module) ensureEventLikes(v goja.Value) ([]goja.Value, bool, error) {
	if v == nil || goja.IsNull(v) || goja.IsUndefined(v) {
		return nil, true, nil
	}

	// Shorthand: string -> {message: "..."}
	if s, ok := v.Export().(string); ok {
		obj := m.vm.NewObject()
		_ = obj.Set("message", s)
		return []goja.Value{obj}, false, nil
	}

	if obj, ok := v.(*goja.Object); ok && obj.ClassName() == "Array" {
		lv := obj.Get("length")
		n := int(lv.ToInteger())
		out := make([]goja.Value, 0, n)
		for i := 0; i < n; i++ {
			it := obj.Get(strconv.Itoa(i))
			evLike, drop, err := m.ensureEventLike(it)
			if err != nil {
				return nil, false, err
			}
			if drop {
				continue
			}
			out = append(out, evLike)
		}
		if len(out) == 0 {
			return nil, true, nil
		}
		return out, false, nil
	}

	evLike, drop, err := m.ensureEventLike(v)
	if err != nil {
		return nil, false, err
	}
	if drop {
		return nil, true, nil
	}
	return []goja.Value{evLike}, false, nil
}

func (m *Module) normalizeEvent(v goja.Value, source, raw string, lineNumber int64) (*Event, error) {
	obj := v.ToObject(m.vm)

	ev := &Event{
		Level:      "INFO",
		Message:    raw,
		Fields:     map[string]any{},
		Tags:       []string{},
		Source:     source,
		Raw:        raw,
		LineNumber: lineNumber,
	}

	if levelVal := obj.Get("level"); !isNullish(levelVal) {
		ev.Level = levelVal.String()
	}
	if msgVal := obj.Get("message"); !isNullish(msgVal) {
		ev.Message = msgVal.String()
	}

	if tsVal := obj.Get("timestamp"); !isNullish(tsVal) {
		ts, err := m.timestampToString(tsVal)
		if err != nil {
			return nil, err
		}
		ev.Timestamp = ts
	}

	if tagsVal := obj.Get("tags"); !isNullish(tagsVal) {
		if arr, ok := tagsVal.Export().([]any); ok {
			out := make([]string, 0, len(arr))
			for _, it := range arr {
				if s, ok := it.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			ev.Tags = out
		}
	}

	if fieldsVal := obj.Get("fields"); !isNullish(fieldsVal) {
		if m2, ok := fieldsVal.Export().(map[string]any); ok {
			for k, v := range m2 {
				ev.Fields[k] = v
			}
		}
	}

	exported, ok := obj.Export().(map[string]any)
	if ok {
		for k, v := range exported {
			switch k {
			case "timestamp", "level", "message", "fields", "tags", "source", "raw", "lineNumber":
				continue
			default:
				if _, exists := ev.Fields[k]; exists {
					continue
				}
				ev.Fields[k] = v
			}
		}
	}

	return ev, nil
}

func (m *Module) timestampToString(v goja.Value) (*string, error) {
	if v == nil {
		return nil, nil
	}
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return nil, nil
	}

	if s, ok := v.Export().(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		return &s, nil
	}

	if obj, ok := v.(*goja.Object); ok {
		if fn, ok := goja.AssertFunction(obj.Get("toISOString")); ok {
			out, err := fn(obj, nil)
			if err != nil {
				return nil, err
			}
			s := strings.TrimSpace(out.String())
			if s == "" {
				return nil, nil
			}
			return &s, nil
		}
	}

	if t, ok := v.Export().(time.Time); ok {
		s := t.UTC().Format(time.RFC3339Nano)
		return &s, nil
	}

	s := strings.TrimSpace(v.String())
	if s == "" {
		return nil, nil
	}
	return &s, nil
}

func enableConsole(vm *goja.Runtime) {
	obj := vm.NewObject()

	_ = obj.Set("log", func(call goja.FunctionCall) goja.Value {
		_, _ = fmt.Fprintln(os.Stdout, joinArgs(call.Arguments)...)
		return goja.Undefined()
	})

	_ = obj.Set("warn", func(call goja.FunctionCall) goja.Value {
		_, _ = fmt.Fprintln(os.Stderr, joinArgs(call.Arguments)...)
		return goja.Undefined()
	})

	_ = obj.Set("error", func(call goja.FunctionCall) goja.Value {
		_, _ = fmt.Fprintln(os.Stderr, joinArgs(call.Arguments)...)
		return goja.Undefined()
	})

	_ = vm.Set("console", obj)
}

func joinArgs(args []goja.Value) []any {
	out := make([]any, 0, len(args))
	for _, a := range args {
		out = append(out, a.Export())
	}
	return out
}

func isNullish(v goja.Value) bool {
	if v == nil {
		return true
	}
	return goja.IsUndefined(v) || goja.IsNull(v)
}

func isInterruptedByTimeout(err error) bool {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		if v, ok := interrupted.Value().(error); ok && errors.Is(v, ErrHookTimeout) {
			return true
		}
	}
	return errors.Is(err, ErrHookTimeout)
}

func (m *Module) newErrorRecord(hook string, err error, source string, lineNumber int64, rawLine *string) *ErrorRecord {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &ErrorRecord{
		Module:     m.Name(),
		Tag:        m.Tag(),
		Hook:       hook,
		Source:     source,
		LineNumber: lineNumber,
		Timeout:    isInterruptedByTimeout(err),
		Message:    msg,
		RawLine:    rawLine,
	}
}

func injectGoHelpers(m *Module) error {
	logVal := m.vm.Get("log")
	if isNullish(logVal) {
		return errors.New("logjs: helpers did not define globalThis.log")
	}
	logObj := logVal.ToObject(m.vm)

	// log.parseTimestamp(value, formats?)
	//
	// - If formats is provided, treat it as a list of Go time.Parse layouts.
	// - Otherwise, use dateparse.ParseAny for best-effort parsing.
	// - Returns a JS Date object or null.
	if err := logObj.Set("parseTimestamp", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 || isNullish(call.Arguments[0]) {
			return goja.Null()
		}

		v := call.Arguments[0].Export()
		var (
			t   time.Time
			ok  bool
			err error
		)

		switch vv := v.(type) {
		case time.Time:
			t, ok = vv, true
		case string:
			s := strings.TrimSpace(vv)
			if s == "" {
				return goja.Null()
			}
			if len(call.Arguments) >= 2 && !isNullish(call.Arguments[1]) {
				if formats, ok2 := call.Arguments[1].Export().([]any); ok2 {
					for _, it := range formats {
						layout, ok3 := it.(string)
						if !ok3 || strings.TrimSpace(layout) == "" {
							continue
						}
						tt, e := time.Parse(layout, s)
						if e == nil {
							t, ok = tt, true
							break
						}
					}
				}
			}
			if !ok {
				tt, e := dateparse.ParseAny(s)
				if e != nil {
					return goja.Null()
				}
				t, ok = tt, true
			}
		case int64:
			t, ok = time.UnixMilli(vv).UTC(), true
		case float64:
			t, ok = time.UnixMilli(int64(vv)).UTC(), true
		default:
			// Try numeric string fallback.
			s := strings.TrimSpace(call.Arguments[0].String())
			if s == "" {
				return goja.Null()
			}
			if i, e := strconv.ParseInt(s, 10, 64); e == nil {
				// Heuristic: if it looks like seconds, treat as seconds.
				if i > 0 && i < 1_000_000_000_000 {
					t, ok = time.Unix(i, 0).UTC(), true
				} else {
					t, ok = time.UnixMilli(i).UTC(), true
				}
			} else {
				tt, e := dateparse.ParseAny(s)
				if e != nil {
					return goja.Null()
				}
				t, ok = tt, true
			}
		}

		if !ok || err != nil {
			return goja.Null()
		}

		return m.newDate(t.UTC())
	}); err != nil {
		return errors.Wrap(err, "set log.parseTimestamp")
	}

	return nil
}
