package styles

// Status icons
const (
	IconSuccess   = "✓"
	IconError     = "✗"
	IconWarning   = "⚠"
	IconInfo      = "ℹ"
	IconRunning   = "▶"
	IconPending   = "○"
	IconSkipped   = "⊘"
	IconSystem    = "●"
	IconGear      = "⚙"
	IconBullet    = "•"
	IconHealthy   = "●" // Green filled circle for healthy
	IconUnhealthy = "●" // Red filled circle for unhealthy
	IconUnknown   = "○" // Empty circle for unknown
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

// HealthIcon returns the appropriate icon for a health status.
func HealthIcon(status string) string {
	switch status {
	case "healthy":
		return IconHealthy
	case "unhealthy":
		return IconUnhealthy
	default:
		return IconUnknown
	}
}

// EventIcon returns the appropriate icon for an event type.
func EventIcon(eventType string) string {
	switch eventType {
	case "success", "completed", "started":
		return IconSuccess
	case "error", "failed":
		return IconError
	case "warning":
		return IconWarning
	case "info":
		return IconInfo
	default:
		return IconBullet
	}
}

