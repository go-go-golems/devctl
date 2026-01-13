package logjs

type Event struct {
	Timestamp  *string        `json:"timestamp,omitempty"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	Fields     map[string]any `json:"fields"`
	Tags       []string       `json:"tags"`
	Source     string         `json:"source"`
	Raw        string         `json:"raw"`
	LineNumber int64          `json:"lineNumber"`
}

type Stats struct {
	LinesProcessed int64
	EventsEmitted  int64
	LinesDropped   int64
	HookErrors     int64
	HookTimeouts   int64
}

type Options struct {
	HookTimeout string
}

type ModuleInfo struct {
	Name         string
	Tag          string
	HasParse     bool
	HasFilter    bool
	HasTransform bool
	HasInit      bool
	HasShutdown  bool
	HasOnError   bool
}

type ErrorRecord struct {
	Module     string  `json:"module"`
	Tag        string  `json:"tag"`
	Hook       string  `json:"hook"`
	Source     string  `json:"source"`
	LineNumber int64   `json:"lineNumber"`
	Timeout    bool    `json:"timeout"`
	Message    string  `json:"message"`
	RawLine    *string `json:"rawLine,omitempty"`
}
