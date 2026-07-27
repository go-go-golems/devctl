package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-go-golems/devctl/cmd/devctl/cmds"
	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/stretchr/testify/require"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    int
		wantMessage string
	}{
		{
			name:        "cobra usage",
			err:         errors.New(`unknown command "removed"`),
			wantCode:    2,
			wantMessage: `E_USAGE: unknown command "removed"`,
		},
		{
			name:        "explicit usage",
			err:         errors.New("E_USAGE: invalid selector"),
			wantCode:    2,
			wantMessage: "E_USAGE: invalid selector",
		},
		{
			name:        "operator runtime",
			err:         &operator.OperatorError{Code: operator.CodePartialFailure, Message: "one failed"},
			wantCode:    1,
			wantMessage: "E_PARTIAL_FAILURE: one failed",
		},
		{
			name:        "unknown service selector",
			err:         &operator.OperatorError{Code: operator.CodeServiceUnknown, Message: "missing"},
			wantCode:    2,
			wantMessage: "E_SERVICE_UNKNOWN: missing",
		},
		{
			name: "plugin exit",
			err: &cmds.PluginCommandExitError{
				Command: "seed", ProviderID: "demo", ExitCode: 42,
			},
			wantCode:    42,
			wantMessage: `plugin command "seed" from provider "demo" failed with exit_code=42`,
		},
		{
			name:        "interrupted",
			err:         context.Canceled,
			wantCode:    130,
			wantMessage: "E_CANCELED: operation canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, message := classifyError(tt.err)
			require.Equal(t, tt.wantCode, code)
			require.Equal(t, tt.wantMessage, message)
		})
	}
}

func TestRenderErrorWritesOneLine(t *testing.T) {
	var output strings.Builder
	code := renderError(&output, errors.New("boom"))
	require.Equal(t, 1, code)
	require.Equal(t, "Error: boom\n", output.String())
}
