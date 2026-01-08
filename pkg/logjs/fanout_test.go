package logjs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTempScriptNamed(t *testing.T, dir, filename, contents string) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(p, []byte(contents), 0o644))
	return p
}

func TestFanout_TagInjection_DefaultTagIsName(t *testing.T) {
	tmp := t.TempDir()

	s1 := writeTempScriptNamed(t, tmp, "a.js", `
register({
  name: "a",
  parse(line, ctx) { return { message: "x" }; },
});
`)
	s2 := writeTempScriptNamed(t, tmp, "b.js", `
register({
  name: "b",
  parse(line, ctx) { return null; },
});
`)

	f, err := LoadFanoutFromFiles(context.Background(), []string{s1, s2}, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close(context.Background()) })

	evs, errs, err := f.ProcessLine(context.Background(), "hi\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 1)
	require.Equal(t, "x", evs[0].Message)
	require.Contains(t, evs[0].Tags, "a")
	require.Equal(t, "a", evs[0].Fields["_tag"])
	require.Equal(t, "a", evs[0].Fields["_module"])
}

func TestFanout_TagInjection_ExplicitTag(t *testing.T) {
	tmp := t.TempDir()

	s1 := writeTempScriptNamed(t, tmp, "a.js", `
register({
  name: "a",
  tag: "errors",
  parse(line, ctx) { return { message: "x" }; },
});
`)

	f, err := LoadFanoutFromFiles(context.Background(), []string{s1}, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close(context.Background()) })

	evs, errs, err := f.ProcessLine(context.Background(), "hi\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 1)
	require.Contains(t, evs[0].Tags, "errors")
	require.Equal(t, "errors", evs[0].Fields["_tag"])
	require.Equal(t, "a", evs[0].Fields["_module"])
}

func TestFanout_ErrorIsolation(t *testing.T) {
	tmp := t.TempDir()

	sOk := writeTempScriptNamed(t, tmp, "ok.js", `
register({
  name: "ok",
  parse(line, ctx) { return { message: "ok:" + line.trim() }; },
});
`)

	sFlaky := writeTempScriptNamed(t, tmp, "flaky.js", `
register({
  name: "flaky",
  parse(line, ctx) {
    if (line.indexOf("bad") >= 0) throw new Error("boom");
    return { message: "flaky:" + line.trim() };
  },
  onError(err, payload, ctx) {
    // swallow
  },
});
`)

	f, err := LoadFanoutFromFiles(context.Background(), []string{sOk, sFlaky}, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close(context.Background()) })

	evs, errs, err := f.ProcessLine(context.Background(), "good\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 2)

	evs, errs, err = f.ProcessLine(context.Background(), "bad\n", "stdin", 2)
	require.NoError(t, err)
	require.Len(t, errs, 1)
	require.Len(t, evs, 1)
	require.Equal(t, "ok:bad", evs[0].Message)
	require.Equal(t, "ok", evs[0].Fields["_module"])
}

func TestFanout_StateIsolation(t *testing.T) {
	tmp := t.TempDir()

	sA := writeTempScriptNamed(t, tmp, "a.js", `
register({
  name: "a",
  parse(line, ctx) {
    ctx.state.n = (ctx.state.n || 0) + 1;
    return { message: "a", n: ctx.state.n };
  },
});
`)

	sB := writeTempScriptNamed(t, tmp, "b.js", `
register({
  name: "b",
  parse(line, ctx) {
    ctx.state.n = (ctx.state.n || 0) + 1;
    return { message: "b", n: ctx.state.n };
  },
});
`)

	f, err := LoadFanoutFromFiles(context.Background(), []string{sA, sB}, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close(context.Background()) })

	evs, errs, err := f.ProcessLine(context.Background(), "x\n", "stdin", 1)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 2)
	n1 := map[string]int64{}
	for _, ev := range evs {
		n1[ev.Fields["_module"].(string)] = ev.Fields["n"].(int64)
	}
	require.Equal(t, int64(1), n1["a"])
	require.Equal(t, int64(1), n1["b"])

	evs, errs, err = f.ProcessLine(context.Background(), "y\n", "stdin", 2)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, evs, 2)
	n2 := map[string]int64{}
	for _, ev := range evs {
		n2[ev.Fields["_module"].(string)] = ev.Fields["n"].(int64)
	}
	require.Equal(t, int64(2), n2["a"])
	require.Equal(t, int64(2), n2["b"])
}
