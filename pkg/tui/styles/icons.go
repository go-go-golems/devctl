package styles

// Status icons
const (
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "⚠"
	IconInfo    = "ℹ"
	IconRunning = "▶"
	IconPending = "○"
	IconSkipped = "⊘"
	IconSystem  = "●"
	IconGear    = "⚙"
	IconBullet  = "•"
)

// StatusIcon returns the appropriate icon for a service status.
func StatusIcon(alive bool) string {
	if alive {
		return IconSuccess
	}
	return IconError
}

// PhaseIcon returns the appropriate icon for a pipeline phase state.
func PhaseIcon(ok *bool, running bool) string {
	if ok == nil {
		if running {
			return IconRunning
		}
		return IconPending
	}
	if *ok {
		return IconSuccess
	}
	return IconError
}

// LogLevelIcon returns the appropriate icon for a log level.
func LogLevelIcon(level string) string {
	switch level {
	case "error", "ERROR":
		return IconError
	case "warn", "WARN", "warning", "WARNING":
		return IconWarning
	case "info", "INFO":
		return IconInfo
	case "debug", "DEBUG":
		return IconBullet
	default:
		return IconBullet
	}
}

