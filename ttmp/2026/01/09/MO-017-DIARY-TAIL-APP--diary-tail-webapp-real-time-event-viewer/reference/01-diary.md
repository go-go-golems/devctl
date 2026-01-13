---
Title: Diary
Type: reference
Ticket: MO-017-DIARY-TAIL-APP
Status: active
Intent: long-term
Topics:
  - webapp
  - tui
  - diary
Created: 2026-01-09
Updated: 2026-01-09
Summary: Implementation diary for the diary tail webapp
RelatedFiles: []
---

# Diary

## Goal

Build a self-contained webapp that watches diary.md files in ttmp directories and streams section update events to a web UI in real-time.

## Step 1: Initial Implementation

Created the core webapp with file watching and SSE streaming. The app uses fsnotify for recursive directory watching, parses markdown headers to track sections, and broadcasts events when new sections are detected.

### What I did

- Created Go webapp with embedded templates
- Implemented recursive diary.md file discovery
- Added section tracking using markdown header parsing
- Set up SSE (Server-Sent Events) for real-time browser updates
- Designed dark-themed Bootstrap UI with event cards
- Extracted ticket metadata from directory names

### Why

- Need real-time visibility into diary updates across multiple tickets
- SSE provides simpler client-side handling than WebSockets for one-way data
- Embedded templates make the app self-contained (single binary)

### What worked

- fsnotify handles recursive directory watching well
- Section diffing reliably detects new headers
- Bootstrap dark theme provides good readability

### Technical details

```
app/
├── main.go          # Core server + file watcher
├── templates/
│   └── index.html   # Bootstrap UI with SSE client
├── static/
│   └── .gitkeep
├── go.mod           # Module definition
├── run.sh           # Quick start script
└── README.md        # Usage docs
```

**Key components:**
- `DiaryWatcher`: manages fsnotify, section tracking, and event broadcasting
- `Server`: HTTP handlers for UI and SSE endpoint
- Event types: `new_section`, `created`, `modified`

### Code review instructions

- Start with `main.go`, review the `DiaryWatcher` struct and `eventLoop`
- Check `templates/index.html` for SSE client implementation
- Run with `./run.sh` and verify events appear when editing diary.md files
