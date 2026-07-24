package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/go-go-golems/devctl/cmd/devctl/cmds"
	devctldoc "github.com/go-go-golems/devctl/pkg/doc"
	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:           "devctl",
	Short:         "devctl is a dev environment orchestrator",
	Version:       version,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromCobra(cmd)
	},
}

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if err := configureRoot(args); err != nil {
		return renderError(os.Stderr, err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	rootCmd.SetArgs(args[1:])
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		return renderError(os.Stderr, err)
	}
	return 0
}

func configureRoot(args []string) error {
	if err := logging.AddLoggingSectionToRootCommand(rootCmd, "devctl"); err != nil {
		return err
	}
	helpSystem := help.NewHelpSystem()
	if err := devctldoc.AddDocToHelpSystem(helpSystem); err != nil {
		return err
	}
	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	if err := cmds.AddCommands(rootCmd); err != nil {
		return err
	}
	return cmds.AddDynamicPluginCommands(rootCmd, args)
}

func renderError(output io.Writer, err error) int {
	code, message := classifyError(err)
	_, _ = fmt.Fprintf(output, "Error: %s\n", message)
	return code
}

func classifyError(err error) (int, string) {
	if errors.Is(err, context.Canceled) {
		return 130, operator.CodeCanceled + ": operation canceled"
	}

	var operatorErr *operator.OperatorError
	if errors.As(err, &operatorErr) {
		switch operatorErr.Code {
		case operator.CodeCanceled:
			return 130, operatorErr.Error()
		case operator.CodeUsage, operator.CodeConfigMissing, operator.CodeConfigInvalid,
			operator.CodeStateVersion, operator.CodeStateCorrupt:
			return 2, operatorErr.Error()
		default:
			return 1, operatorErr.Error()
		}
	}

	message := err.Error()
	if strings.Contains(message, operator.CodeUsage+":") {
		return 2, message
	}
	if isCobraUsageError(message) {
		return 2, operator.CodeUsage + ": " + message
	}
	if strings.Contains(message, "plugin command catalog unavailable") {
		return 2, operator.CodeConfigInvalid + ": " + message
	}
	return 1, message
}

func isCobraUsageError(message string) bool {
	usageFragments := []string{
		"unknown command ",
		"unknown flag:",
		"required flag(s)",
		"requires at least",
		"requires at most",
		"accepts ",
	}
	for _, fragment := range usageFragments {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
