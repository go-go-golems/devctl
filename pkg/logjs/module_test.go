package logjs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTempScript(t *testing.T, dir, contents string) string {
	t.Helper()
	p := filepath.Join(dir, "parser.js")
	require.NoError(t, os.WriteFile(p, []byte(contents), 0o644))
	return p
}

func TestModule_Parse_Filter_Transform(t *testing.T) {
	tmp := t.TempDir()

	scriptPath := writeTempScript(t, tmp, `
register({
  name: "t",
  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;
    return { message: obj.msg, level: obj.level, trace_id: obj.trace_id, tags: ["a",""] };
  },
  filter(event, ctx) { return event.level !== "DEBUG"; },
  transform(event, ctx) { event.fields = { x: 1 }; return event; },
});
`)

	m, err := LoadFromFile(context.Background(), scriptPath, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close(context.Background()) })

	evs, errs, err := m.ProcessLine(context.Background(), `{"msg":"hi","level":"INFO","trace_id":"abc"}`+"\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 1)
	ev := evs[0]
	require.Equal(t, "INFO", ev.Level)
	require.Equal(t, "hi", ev.Message)
	require.Equal(t, "stdin", ev.Source)
	require.Equal(t, `{"msg":"hi","level":"INFO","trace_id":"abc"}`, ev.Raw)
	require.Equal(t, int64(1), ev.LineNumber)
	require.Equal(t, []string{"a"}, ev.Tags)
	require.Equal(t, map[string]any{"x": int64(1), "trace_id": "abc"}, ev.Fields)

	evs, errs, err = m.ProcessLine(context.Background(), `{"msg":"no","level":"DEBUG"}`+"\n", "stdin", 2)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Empty(t, evs)

	st := m.Stats()
	require.Equal(t, int64(2), st.LinesProcessed)
	require.Equal(t, int64(1), st.EventsEmitted)
	require.Equal(t, int64(1), st.LinesDropped)
}

func TestModule_Timestamp_Date_ToISOString(t *testing.T) {
	tmp := t.TempDir()

	scriptPath := writeTempScript(t, tmp, `
register({
  name: "t",
  parse(line, ctx) {
    return { timestamp: new Date("2020-01-01T00:00:00Z"), message: "x" };
  },
});
`)

	m, err := LoadFromFile(context.Background(), scriptPath, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close(context.Background()) })

	evs, errs, err := m.ProcessLine(context.Background(), "ignored\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 1)
	ev := evs[0]
	require.NotNil(t, ev.Timestamp)
	require.Equal(t, "2020-01-01T00:00:00.000Z", *ev.Timestamp)
}

func TestModule_Timeout(t *testing.T) {
	tmp := t.TempDir()

	scriptPath := writeTempScript(t, tmp, `
register({
  name: "t",
  parse(line, ctx) {
    while(true) {}
  },
  onError(err, payload, ctx) {
    // swallow
  }
});
`)

	m, err := LoadFromFile(context.Background(), scriptPath, Options{HookTimeout: "10ms"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close(context.Background()) })

	evs, errs, err := m.ProcessLine(context.Background(), "x\n", "stdin", 1)
	require.NoError(t, err)
	require.Len(t, errs, 1)
	require.Empty(t, evs)

	st := m.Stats()
	require.Equal(t, int64(1), st.HookTimeouts)
	require.Equal(t, int64(1), st.HookErrors)
}

func TestModule_Parse_ReturnArray_Transform_ReturnArray(t *testing.T) {
	tmp := t.TempDir()

	scriptPath := writeTempScript(t, tmp, `
register({
  name: "t",
  parse(line, ctx) {
    if (line.trim() !== "x") return null;
    return ["a", { message: "b" }];
  },
  transform(event, ctx) {
    if (event.message === "a") return [event, { message: "c" }];
    return event;
  }
});
`)

	m, err := LoadFromFile(context.Background(), scriptPath, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close(context.Background()) })

	evs, errs, err := m.ProcessLine(context.Background(), "x\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 3)
	require.Equal(t, "a", evs[0].Message)
	require.Equal(t, "c", evs[1].Message)
	require.Equal(t, "b", evs[2].Message)
}

func TestModule_ErrorRecord_OnParseThrow(t *testing.T) {
	tmp := t.TempDir()

	scriptPath := writeTempScript(t, tmp, `
register({
  name: "t",
  parse(line, ctx) { throw new Error("boom"); },
  onError(err, payload, ctx) {
    // swallow
  }
});
`)

	m, err := LoadFromFile(context.Background(), scriptPath, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close(context.Background()) })

	evs, errs, err := m.ProcessLine(context.Background(), "x\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, evs)
	require.Len(t, errs, 1)
	require.Equal(t, "parse", errs[0].Hook)
	require.Equal(t, "t", errs[0].Module)
	require.Equal(t, "t", errs[0].Tag)
	require.False(t, errs[0].Timeout)
}

func TestModule_Helper_ParseTimestamp_ToISOString(t *testing.T) {
	tmp := t.TempDir()

	scriptPath := writeTempScript(t, tmp, `
register({
  name: "t",
  parse(line, ctx) {
    return { timestamp: log.parseTimestamp("2020-01-01T00:00:00Z"), message: "x" };
  },
});
`)

	m, err := LoadFromFile(context.Background(), scriptPath, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close(context.Background()) })

	evs, errs, err := m.ProcessLine(context.Background(), "ignored\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 1)
	require.NotNil(t, evs[0].Timestamp)
	require.Equal(t, "2020-01-01T00:00:00.000Z", *evs[0].Timestamp)
}

func TestModule_Helper_ParseTimestamp_Numeric(t *testing.T) {
	tmp := t.TempDir()

	scriptPath := writeTempScript(t, tmp, `
register({
  name: "t",
  parse(line, ctx) {
    return [
      { timestamp: log.parseTimestamp(1700000000), message: "sec" },
      { timestamp: log.parseTimestamp(1700000000000), message: "ms" }
    ];
  },
});
`)

	m, err := LoadFromFile(context.Background(), scriptPath, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close(context.Background()) })

	evs, errs, err := m.ProcessLine(context.Background(), "ignored\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 2)
	require.Equal(t, "2023-11-14T22:13:20.000Z", *evs[0].Timestamp)
	require.Equal(t, "2023-11-14T22:13:20.000Z", *evs[1].Timestamp)
}

func TestModule_Helper_MultilineBuffer_AfterNegate(t *testing.T) {
	tmp := t.TempDir()

	scriptPath := writeTempScript(t, tmp, `
register({
  name: "t",
  init(ctx) {
    ctx.state.buf = log.createMultilineBuffer({
      pattern: /^\s+at /,
      negate: true,
      match: "after",
      maxLines: 50,
    });
  },
  parse(line, ctx) {
    const out = ctx.state.buf.add(line.trimEnd());
    if (!out) return null;
    return { message: out.split("\n")[0], fields: { stack: out } };
  }
});
`)

	m, err := LoadFromFile(context.Background(), scriptPath, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close(context.Background()) })

	evs, errs, err := m.ProcessLine(context.Background(), "E1\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Empty(t, evs)

	evs, errs, err = m.ProcessLine(context.Background(), "  at a\n", "stdin", 2)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Empty(t, evs)

	evs, errs, err = m.ProcessLine(context.Background(), "  at b\n", "stdin", 3)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Empty(t, evs)

	// Encountering the next start line flushes the previous record.
	evs, errs, err = m.ProcessLine(context.Background(), "E2\n", "stdin", 4)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 1)
	require.Equal(t, "E1", evs[0].Message)
	require.Contains(t, evs[0].Fields["stack"].(string), "at a")
}
