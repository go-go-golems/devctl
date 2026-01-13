# Diary Tail App

Real-time event viewer for `diary.md` file updates. Recursively watches a directory for diary files and streams new section events to a web UI.

## Features

- 🔍 Recursive file watching for `diary.md` files
- 📡 Real-time SSE (Server-Sent Events) push to browser
- 🎨 Dark theme with Bootstrap styling
- 📋 Ticket metadata extraction from directory names
- ⚡ Debounced file change detection

## Quick Start

```bash
# Install dependencies
go mod tidy

# Run watching the ttmp directory
go run main.go -dir /path/to/ttmp

# Or build and run
go build -o diary-tail
./diary-tail -dir /path/to/ttmp -port 8765
```

Then open http://localhost:8765 in your browser.

## Command Line Options

| Flag   | Default | Description                              |
|--------|---------|------------------------------------------|
| `-dir` | `.`     | Root directory to watch for diary files  |
| `-port`| `8765`  | HTTP server port                         |

## How It Works

1. **File Discovery**: On startup, recursively finds all `diary.md` files
2. **Section Tracking**: Parses markdown headers (# ## ###) from each file
3. **Watch Loop**: Uses fsnotify to detect file changes
4. **Change Detection**: Compares sections before/after to find new ones
5. **Event Broadcast**: Sends new section events to all connected browsers via SSE

## Event Types

- `new_section` - A new markdown header was added to a diary
- `created` - A new diary.md file was created
- `modified` - Content changed (but no new sections)

## API Endpoints

| Endpoint      | Description                    |
|---------------|--------------------------------|
| `GET /`       | Web UI                         |
| `GET /events` | SSE stream for real-time events|
| `GET /api/events` | JSON list of recent events |

## Directory Structure

The app expects ticket directories in the format:
```
ttmp/YYYY/MM/DD/TICKET-ID--slug-title/
  └── diary.md
```

Ticket metadata is extracted from the directory name.

