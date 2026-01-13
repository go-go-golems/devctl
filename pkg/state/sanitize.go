package state

import (
	"strings"
)

// sensitiveKeyPatterns contains patterns that indicate a key holds sensitive data.
var sensitiveKeyPatterns = []string{
	"PASSWORD",
	"SECRET",
	"TOKEN",
	"KEY",
	"CREDENTIAL",
	"API_KEY",
	"APIKEY",
	"AUTH",
	"PRIVATE",
	"CERT",
	"PASSPHRASE",
}

// redactedValue is the placeholder for redacted values.
const redactedValue = "[REDACTED]"

// SanitizeEnv returns a copy of the environment map with sensitive values redacted.
// Keys containing patterns like PASSWORD, SECRET, TOKEN, KEY, CREDENTIAL, etc.
// will have their values replaced with "[REDACTED]".
func SanitizeEnv(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}

	result := make(map[string]string, len(env))
	for k, v := range env {
		if isSensitiveKey(k) {
			result[k] = redactedValue
		} else {
			result[k] = v
		}
	}
	return result
}

// isSensitiveKey checks if a key name indicates sensitive data.
func isSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, pattern := range sensitiveKeyPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// FilterEnvForDisplay returns a subset of environment variables suitable for display.
// It removes common noise variables and limits the total count.
func FilterEnvForDisplay(env map[string]string, maxVars int) map[string]string {
	if env == nil {
		return nil
	}

	// Skip noisy/uninteresting variables
	skipPrefixes := []string{
		"_",            // Shell internals
		"LESS",         // Pager settings
		"LS_COLORS",    // ls coloring
		"BASH",         // Bash internals
		"SHELL",        // Already known
		"TERM_PROGRAM", // Terminal metadata
	}

	skipExact := map[string]bool{
		"PWD":      true, // We show cwd separately
		"OLDPWD":   true,
		"SHLVL":    true,
		"HOME":     true, // Well-known
		"USER":     true,
		"LOGNAME":  true,
		"HOSTNAME": true,
		"LANG":     true,
		"LC_ALL":   true,
	}

	result := make(map[string]string)
	for k, v := range env {
		if skipExact[k] {
			continue
		}

		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(k, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		result[k] = v
		if maxVars > 0 && len(result) >= maxVars {
			break
		}
	}

	return result
}
