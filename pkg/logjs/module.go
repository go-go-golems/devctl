package logjs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

var ErrNoRegister = errors.New("logjs: script did not call register()")
var ErrHookTimeout = errors.New("logjs: js hook timeout")

type Module struct {
	vm     *goja.Runtime
	opts   options
	config *goja.Object

	name string

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
		vm:    goja.New(),
		opts:  parsedOpts,
		state: nil,
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
	if goja.IsUndefined(nameVal) || goja.IsNull(nameVal) || strings.TrimSpace(nameVal.String()) == "" {
		return nil, errors.New("register({ name: string, ... }): name is required")
	}
	m.name = nameVal.String()

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

func (m *Module) Stats() Stats {
	return m.stats
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

func (m *Module) ProcessLine(ctx context.Context, line string, source string, lineNumber int64) (*Event, error) {
	_ = ctx

	m.stats.LinesProcessed++

	trimmed := trimTrailingNewline(line)
	ctxObj := m.buildContext("parse", source, lineNumber)

	v, err := m.callHook("parse", m.parseFn, m.vm.ToValue(trimmed), ctxObj)
	if err != nil {
		m.stats.HookErrors++
		m.callOnError("parse", err, m.vm.ToValue(trimmed), ctxObj)
		m.stats.LinesDropped++
		return nil, nil
	}

	eventLike, drop, err := m.ensureEventLike(v)
	if err != nil {
		m.stats.HookErrors++
		m.callOnError("parse", err, m.vm.ToValue(trimmed), ctxObj)
		m.stats.LinesDropped++
		return nil, nil
	}
	if drop {
		m.stats.LinesDropped++
		return nil, nil
	}

	if m.filterFn != nil {
		ctxObj = m.buildContext("filter", source, lineNumber)
		keepVal, err := m.callHook("filter", m.filterFn, eventLike, ctxObj)
		if err != nil {
			m.stats.HookErrors++
			m.callOnError("filter", err, eventLike, ctxObj)
			m.stats.LinesDropped++
			return nil, nil
		}
		if !keepVal.ToBoolean() {
			m.stats.LinesDropped++
			return nil, nil
		}
	}

	if m.transformFn != nil {
		ctxObj = m.buildContext("transform", source, lineNumber)
		outVal, err := m.callHook("transform", m.transformFn, eventLike, ctxObj)
		if err != nil {
			m.stats.HookErrors++
			m.callOnError("transform", err, eventLike, ctxObj)
			m.stats.LinesDropped++
			return nil, nil
		}
		eventLike, drop, err = m.ensureEventLike(outVal)
		if err != nil {
			m.stats.HookErrors++
			m.callOnError("transform", err, outVal, ctxObj)
			m.stats.LinesDropped++
			return nil, nil
		}
		if drop {
			m.stats.LinesDropped++
			return nil, nil
		}
	}

	ev, err := m.normalizeEvent(eventLike, source, trimmed, lineNumber)
	if err != nil {
		m.stats.HookErrors++
		ctxObj = m.buildContext("transform", source, lineNumber)
		m.callOnError("transform", err, eventLike, ctxObj)
		m.stats.LinesDropped++
		return nil, nil
	}

	m.stats.EventsEmitted++
	return ev, nil
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
