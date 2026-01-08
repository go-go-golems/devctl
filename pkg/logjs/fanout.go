package logjs

import (
	"context"
	"strings"

	"github.com/pkg/errors"
)

// Fanout runs multiple independent JS modules against the same input stream.
// Each emitted event is tagged with the module's tag/name.
type Fanout struct {
	Modules []*Module
}

func LoadFanoutFromFiles(ctx context.Context, scriptPaths []string, opts Options) (*Fanout, error) {
	_ = ctx

	if len(scriptPaths) == 0 {
		return nil, errors.New("logjs: at least one module script is required")
	}

	out := &Fanout{Modules: make([]*Module, 0, len(scriptPaths))}
	for _, p := range scriptPaths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		m, err := LoadFromFile(ctx, p, opts)
		if err != nil {
			_ = out.Close(ctx)
			return nil, err
		}
		out.Modules = append(out.Modules, m)
	}
	if len(out.Modules) == 0 {
		return nil, errors.New("logjs: at least one module script is required")
	}

	return out, nil
}

func (f *Fanout) Close(ctx context.Context) error {
	var firstErr error
	for _, m := range f.Modules {
		if m == nil {
			continue
		}
		if err := m.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (f *Fanout) ProcessLine(ctx context.Context, line string, source string, lineNumber int64) ([]*Event, []*ErrorRecord, error) {
	out := make([]*Event, 0, len(f.Modules))
	outErrs := make([]*ErrorRecord, 0)
	for _, m := range f.Modules {
		if m == nil {
			continue
		}

		events, errs, err := m.ProcessLine(ctx, line, source, lineNumber)
		if err != nil {
			return nil, nil, err
		}
		outErrs = append(outErrs, errs...)
		for _, ev := range events {
			if ev == nil {
				continue
			}
			injectTag(ev, m.Tag(), m.Name())
			out = append(out, ev)
		}
	}
	return out, outErrs, nil
}

func injectTag(ev *Event, tag string, moduleName string) {
	if ev == nil {
		return
	}

	tag = strings.TrimSpace(tag)
	if tag != "" {
		if !containsString(ev.Tags, tag) {
			ev.Tags = append(ev.Tags, tag)
		}
		if ev.Fields == nil {
			ev.Fields = map[string]any{}
		}
		if _, ok := ev.Fields["_tag"]; !ok {
			ev.Fields["_tag"] = tag
		}
	}

	moduleName = strings.TrimSpace(moduleName)
	if moduleName != "" {
		if ev.Fields == nil {
			ev.Fields = map[string]any{}
		}
		if _, ok := ev.Fields["_module"]; !ok {
			ev.Fields["_module"] = moduleName
		}
	}
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
