# Diary

## Step 1: Initial Implementation

Built a self-contained webapp that tails diary.md files in ttmp directories and streams section update events to a web UI in real-time.

### What I did

- Created Go webapp with embedded templates using `//go:embed`
- Implemented recursive directory watching with fsnotify
- Added section tracking by parsing markdown `##` headers
- Set up SSE (Server-Sent Events) for real-time browser updates
- Designed dark-themed Bootstrap UI with event cards
- Extracted ticket metadata from `TICKET-ID--slug-title` directory names

### Why

- Need real-time visibility into diary updates across multiple tickets
- SSE provides simpler client-side handling than WebSockets for one-way data
- Embedded templates make the app self-contained (single binary deployment)
- Bootstrap dark theme reduces eye strain during extended use

### What worked

- fsnotify handles recursive directory watching well with minimal CPU usage
- Section diffing reliably detects new `##` headers
- SSE reconnection logic in the browser handles server restarts gracefully
- Debouncing (500ms) prevents duplicate events from rapid file saves

### What didn't work

- Initial regex `^#{1,3}\s+` matched all header levels, creating too many events
- Fixed by changing to `^##\s+` to only match section-level headers

### Technical details

```
app/
├── main.go          # Core server + file watcher (~400 lines)
├── templates/
│   └── index.html   # Bootstrap UI with SSE client
├── static/
│   └── .gitkeep
├── go.mod           # Module: diary-tail, requires fsnotify
├── run.sh           # Quick start script
└── README.md        # Usage documentation
```

**Key components:**

| Component | Purpose |
|-----------|---------|
| `DiaryWatcher` | Manages fsnotify, section tracking, event broadcasting |
| `Server` | HTTP handlers for UI (`/`), SSE (`/events`), API (`/api/events`) |
| `parseTicketMeta` | Extracts ticket ID and title from directory path |
| `parseSections` | Reads `##` headers from diary.md files |
| `getSectionContent` | Extracts preview content under a section |

**Event types:**
- `new_section` - A new `##` header was added
- `created` - A new diary.md file was created

### Next steps

- [ ] Add filtering by ticket ID in the UI
- [ ] Add search functionality for event content
- [ ] Support watching multiple root directories
- [ ] Add notification sounds for new events
- [ ] Persist events to a local SQLite database

## Step 2: Added Expandable Sections with Markdown

Enhanced the UI to support expanding event cards to see full section content with proper markdown rendering.

### What I did

- Added `marked.js` library for client-side markdown parsing
- Split content into `Preview` (6 lines) and `FullContent` (full section)
- Added click-to-expand on section headers with chevron animation
- Styled markdown elements: headers, code blocks, tables, lists

### Technical details

```go
// SectionContent holds both preview and full content
type SectionContent struct {
    Preview string
    Full    string
}
```

The markdown is rendered lazily on first expand to avoid processing hidden content.

| Feature | Implementation |
|---------|----------------|
| Expand toggle | Click on section header |
| Markdown lib | marked.js via CDN |
| Lazy render | Only on first expand |
| Styling | Custom CSS for dark theme |

## Step 3: Testing the Expand Feature

This section tests the full markdown rendering capability when the event card is expanded.

### Code blocks

```go
func main() {
    fmt.Println("Hello, Diary Tail!")
}
```

### Lists

- Item one with **bold** text
- Item two with `inline code`
- Item three with *italics*

### Table example

| Column A | Column B | Column C |
|----------|----------|----------|
| Cell 1   | Cell 2   | Cell 3   |
| Cell 4   | Cell 5   | Cell 6   |

> This is a blockquote to test styling.

## Step 4: Split View with Diary Reader

Completely redesigned the UI to a split-view layout with clickable events that open the full diary.

### What I did

- Created two-panel layout: events on left, diary reader on right
- Added `/api/diary` endpoint to fetch full diary content
- Implemented URL state with `?file=...&section=...` parameters
- Added scroll-to-section with highlight animation
- Browser back/forward navigation support

### How it works

1. Click an event in the left panel
2. Full diary loads in the right panel
3. Page scrolls to the clicked section
4. URL updates for sharing/bookmarking

### Features

| Feature | Status |
|---------|--------|
| Split view layout | ✅ |
| Click to open diary | ✅ |
| URL state with file/section | ✅ |
| Full markdown rendering | ✅ |
| Scroll to section | ✅ |
| Section highlight animation | ✅ |
| Browser back/forward | ✅ |
| Mobile responsive | ✅ |

## Step 5: Multi-Ticket Demo

Testing with multiple ticket diaries to show events from different projects in the same feed!
