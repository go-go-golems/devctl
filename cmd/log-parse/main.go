package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/go-go-golems/devctl/pkg/logjs"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var version = "dev"

type options struct {
	modulePaths   []string
	modulesDirs   []string
	inputPath     string
	source        string
	format        string
	jsTimeout     string
	workers       int
	unsafeModules []string
	printPipeline bool
	stats         bool
	errorsOut     string
}

func main() {
	opts := options{}

	rootCmd := &cobra.Command{
		Use:     "log-parse",
		Short:   "Parse log lines with JavaScript modules (goja-based)",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return logging.InitLoggerFromCobra(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cmd, opts)
		},
	}

	cobra.CheckErr(logging.AddLoggingLayerToRootCommand(rootCmd, "log-parse"))

	rootCmd.Flags().StringVar(&opts.inputPath, "input", "", "Input file path (default: stdin)")
	rootCmd.Flags().StringVar(&opts.source, "source", "", "Source label (default: input filename or stdin)")
	rootCmd.Flags().StringVar(&opts.format, "format", "ndjson", "Output format: ndjson|pretty")
	rootCmd.Flags().BoolVar(&opts.printPipeline, "print-pipeline", false, "Print loaded modules and their hooks (to stderr)")
	rootCmd.Flags().BoolVar(&opts.stats, "stats", false, "Print per-module stats on exit (to stderr)")
	rootCmd.Flags().StringVar(&opts.errorsOut, "errors", "", "Write structured error records as NDJSON to this path ('stderr' or '-' for stderr)")

	rootCmd.PersistentFlags().StringSliceVar(&opts.modulePaths, "module", nil, "Path to a JS module file (repeatable)")
	rootCmd.PersistentFlags().StringSliceVar(&opts.modulesDirs, "modules-dir", nil, "Directory to load all *.js files from (repeatable, non-recursive)")
	rootCmd.PersistentFlags().StringVar(&opts.jsTimeout, "js-timeout", "0", "Per-hook JS timeout (e.g. 50ms, 200ms)")
	rootCmd.PersistentFlags().IntVar(&opts.workers, "workers", 1, "Number of JS runtimes/workers (MVP: only 1 supported)")
	rootCmd.PersistentFlags().StringSliceVar(&opts.unsafeModules, "unsafe-modules", nil, "Opt-in unsafe modules (reserved; MVP ignores)")

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate modules without running (compile + register + name uniqueness)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return validate(cmd.Context(), cmd, opts)
		},
	}
	rootCmd.AddCommand(validateCmd)

	cobra.CheckErr(rootCmd.Execute())
}

func run(ctx context.Context, cmd *cobra.Command, opts options) error {
	if opts.workers != 1 {
		return errors.New("MVP supports only --workers=1")
	}
	if opts.format != "ndjson" && opts.format != "pretty" {
		return errors.New("--format must be ndjson or pretty")
	}

	modulePaths, err := resolveModulePaths(opts.modulePaths, opts.modulesDirs)
	if err != nil {
		return err
	}

	errorsWriter, closeErrorsWriter, err := openErrorsWriter(cmd, opts)
	if err != nil {
		return err
	}
	if closeErrorsWriter != nil {
		defer func() { _ = closeErrorsWriter.Close() }()
	}
	var errEnc *json.Encoder
	if errorsWriter != nil {
		errEnc = json.NewEncoder(errorsWriter)
		errEnc.SetEscapeHTML(false)
	}

	var r io.Reader
	var closer io.Closer
	if opts.inputPath != "" {
		f, err := os.Open(opts.inputPath)
		if err != nil {
			return err
		}
		r = f
		closer = f
	} else {
		r = os.Stdin
	}
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}

	source := opts.source
	if source == "" {
		if opts.inputPath != "" {
			source = opts.inputPath
		} else {
			source = "stdin"
		}
	}

	fanout, err := logjs.LoadFanoutFromFiles(ctx, modulePaths, logjs.Options{HookTimeout: opts.jsTimeout})
	if err != nil {
		return err
	}
	defer func() { _ = fanout.Close(ctx) }()

	if err := validateUniqueModuleNames(fanout.Modules); err != nil {
		return err
	}
	if opts.printPipeline {
		printPipeline(cmd.ErrOrStderr(), fanout.Modules)
	}

	out := cmd.OutOrStdout()
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)

	br := bufio.NewReader(r)
	var lineNumber int64
	for {
		line, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if line == "" && errors.Is(err, io.EOF) {
			break
		}

		lineNumber++
		events, errs, perr := fanout.ProcessLine(ctx, line, source, lineNumber)
		if perr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "log-parse: line %d: %v\n", lineNumber, perr)
		}
		for _, er := range errs {
			if er == nil || errEnc == nil {
				continue
			}
			if err := errEnc.Encode(er); err != nil {
				return err
			}
		}
		for _, ev := range events {
			if ev == nil {
				continue
			}
			switch opts.format {
			case "ndjson":
				if err := enc.Encode(ev); err != nil {
					return err
				}
			case "pretty":
				b, err := json.MarshalIndent(ev, "", "  ")
				if err != nil {
					return err
				}
				if _, err := out.Write(append(b, '\n')); err != nil {
					return err
				}
			default:
				return errors.New("unsupported format")
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	if opts.stats {
		printStats(cmd.ErrOrStderr(), fanout.Modules)
	}

	return nil
}

func validate(ctx context.Context, cmd *cobra.Command, opts options) error {
	modulePaths, err := resolveModulePaths(opts.modulePaths, opts.modulesDirs)
	if err != nil {
		return err
	}

	fanout, err := logjs.LoadFanoutFromFiles(ctx, modulePaths, logjs.Options{HookTimeout: opts.jsTimeout})
	if err != nil {
		return err
	}
	defer func() { _ = fanout.Close(ctx) }()

	if err := validateUniqueModuleNames(fanout.Modules); err != nil {
		return err
	}
	printPipeline(cmd.OutOrStdout(), fanout.Modules)
	return nil
}

func openErrorsWriter(cmd *cobra.Command, opts options) (io.Writer, io.Closer, error) {
	if strings.TrimSpace(opts.errorsOut) == "" {
		return nil, nil, nil
	}

	dest := strings.TrimSpace(opts.errorsOut)
	if dest == "-" || strings.EqualFold(dest, "stderr") {
		if opts.printPipeline || opts.stats {
			return nil, nil, errors.New("--errors=stderr conflicts with --print-pipeline/--stats (would mix NDJSON with text)")
		}
		return cmd.ErrOrStderr(), nil, nil
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, nil, errors.Wrap(err, "open --errors file")
	}
	return f, f, nil
}

func resolveModulePaths(explicitPaths []string, dirs []string) ([]string, error) {
	out := make([]string, 0, len(explicitPaths))

	for _, p := range explicitPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			return nil, errors.Wrapf(err, "stat module: %s", p)
		}
		out = append(out, p)
	}

	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, errors.Wrapf(err, "read modules dir: %s", dir)
		}
		jsFiles := make([]string, 0, len(entries))
		for _, ent := range entries {
			if ent.IsDir() {
				continue
			}
			name := ent.Name()
			if !strings.HasSuffix(name, ".js") {
				continue
			}
			jsFiles = append(jsFiles, filepath.Join(dir, name))
		}
		sort.Strings(jsFiles)
		out = append(out, jsFiles...)
	}

	if len(out) == 0 {
		return nil, errors.New("at least one --module or --modules-dir is required")
	}
	return out, nil
}

func validateUniqueModuleNames(modules []*logjs.Module) error {
	seen := map[string]struct{}{}
	for _, m := range modules {
		if m == nil {
			continue
		}
		name := strings.TrimSpace(m.Name())
		if name == "" {
			return errors.New("module name must not be empty")
		}
		if _, ok := seen[name]; ok {
			return errors.Errorf("duplicate module name: %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func printPipeline(w io.Writer, modules []*logjs.Module) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tTAG\tHOOKS\tSOURCE")
	for _, m := range modules {
		if m == nil {
			continue
		}
		info := m.Info()
		hooks := make([]string, 0, 6)
		if info.HasParse {
			hooks = append(hooks, "parse")
		}
		if info.HasFilter {
			hooks = append(hooks, "filter")
		}
		if info.HasTransform {
			hooks = append(hooks, "transform")
		}
		if info.HasInit {
			hooks = append(hooks, "init")
		}
		if info.HasShutdown {
			hooks = append(hooks, "shutdown")
		}
		if info.HasOnError {
			hooks = append(hooks, "onError")
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", info.Name, info.Tag, strings.Join(hooks, ","), m.ScriptPath())
	}
	_ = tw.Flush()
}

func printStats(w io.Writer, modules []*logjs.Module) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "MODULE\tTAG\tLINES\tEMITTED\tDROPPED\tHOOK_ERRORS\tHOOK_TIMEOUTS")
	for _, m := range modules {
		if m == nil {
			continue
		}
		st := m.Stats()
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\t%d\n",
			m.Name(),
			m.Tag(),
			st.LinesProcessed,
			st.EventsEmitted,
			st.LinesDropped,
			st.HookErrors,
			st.HookTimeouts,
		)
	}
	_ = tw.Flush()
}
