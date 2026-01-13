// Diary Tail App - Real-time event viewer for diary.md files
package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// DiaryEvent represents a change event in a diary file
type DiaryEvent struct {
	ID          int       `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	FilePath    string    `json:"filePath"`
	TicketID    string    `json:"ticketId"`
	Title       string    `json:"title"`
	Section     string    `json:"section"`
	Preview     string    `json:"preview"`     // First few lines for collapsed view
	FullContent string    `json:"fullContent"` // Full markdown content for expanded view
	EventType   string    `json:"eventType"`   // "new_section", "modified", "created"
}

// TicketMeta holds parsed ticket metadata
type TicketMeta struct {
	TicketID string `json:"ticketId"`
	Title    string `json:"title"`
	Path     string `json:"path"`
}

// DiaryInfo holds information about a diary file
type DiaryInfo struct {
	FilePath    string    `json:"filePath"`
	TicketID    string    `json:"ticketId"`
	Title       string    `json:"title"`
	LastModified time.Time `json:"lastModified"`
	SectionCount int       `json:"sectionCount"`
}

// DiaryWatcher manages file watching and event broadcasting
type DiaryWatcher struct {
	mu           sync.RWMutex
	watcher      *fsnotify.Watcher
	rootDir      string
	clients      map[chan DiaryEvent]bool
	events       []DiaryEvent
	eventID      int
	fileSections map[string][]string // track sections per file
}

func NewDiaryWatcher(rootDir string) (*DiaryWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dw := &DiaryWatcher{
		watcher:      watcher,
		rootDir:      rootDir,
		clients:      make(map[chan DiaryEvent]bool),
		events:       make([]DiaryEvent, 0),
		fileSections: make(map[string][]string),
	}

	return dw, nil
}

// parseTicketMeta extracts ticket metadata from a directory path
func parseTicketMeta(dirPath string) TicketMeta {
	base := filepath.Base(dirPath)
	parts := strings.SplitN(base, "--", 2)

	meta := TicketMeta{
		Path: dirPath,
	}

	if len(parts) >= 1 {
		meta.TicketID = parts[0]
	}
	if len(parts) >= 2 {
		// Convert slug to title
		meta.Title = strings.ReplaceAll(parts[1], "-", " ")
		meta.Title = strings.Title(meta.Title)
	}

	return meta
}

// parseSections extracts section headers (## level) from a markdown file
func parseSections(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sections []string
	scanner := bufio.NewScanner(file)
	// Only match ## headers (sections), not # (title) or ### (subsections)
	headerRegex := regexp.MustCompile(`^##\s+(.+)$`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := headerRegex.FindStringSubmatch(line); len(matches) > 1 {
			sections = append(sections, matches[1])
		}
	}

	return sections, scanner.Err()
}

// getNewSections compares old and new sections, returning only new ones
func getNewSections(old, new []string) []string {
	oldSet := make(map[string]bool)
	for _, s := range old {
		oldSet[s] = true
	}

	var newSections []string
	for _, s := range new {
		if !oldSet[s] {
			newSections = append(newSections, s)
		}
	}
	return newSections
}

// SectionContent holds both preview and full content
type SectionContent struct {
	Preview string
	Full    string
}

// getSectionContent reads content following a ## section header until the next ## header
func getSectionContent(filePath, sectionTitle string) SectionContent {
	file, err := os.Open(filePath)
	if err != nil {
		return SectionContent{}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Only ## headers mark section boundaries
	headerRegex := regexp.MustCompile(`^##\s+(.+)$`)

	inSection := false
	var fullContent strings.Builder
	var previewLines []string
	maxPreviewLines := 6

	for scanner.Scan() {
		line := scanner.Text()

		if matches := headerRegex.FindStringSubmatch(line); len(matches) > 1 {
			if inSection {
				break // Hit next ## section
			}
			if matches[1] == sectionTitle {
				inSection = true
				continue
			}
		}

		if inSection {
			fullContent.WriteString(line)
			fullContent.WriteString("\n")
			if len(previewLines) < maxPreviewLines {
				previewLines = append(previewLines, line)
			}
		}
	}

	preview := strings.TrimSpace(strings.Join(previewLines, "\n"))
	full := strings.TrimSpace(fullContent.String())

	// Add ellipsis to preview if content was truncated
	if len(strings.Split(full, "\n")) > maxPreviewLines {
		preview += "\n..."
	}

	return SectionContent{
		Preview: preview,
		Full:    full,
	}
}

// Subscribe adds a client to receive events
func (dw *DiaryWatcher) Subscribe() chan DiaryEvent {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	ch := make(chan DiaryEvent, 100)
	dw.clients[ch] = true
	return ch
}

// Unsubscribe removes a client
func (dw *DiaryWatcher) Unsubscribe(ch chan DiaryEvent) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	delete(dw.clients, ch)
	close(ch)
}

// broadcast sends an event to all connected clients
func (dw *DiaryWatcher) broadcast(event DiaryEvent) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	dw.eventID++
	event.ID = dw.eventID
	dw.events = append(dw.events, event)

	// Keep only last 100 events
	if len(dw.events) > 100 {
		dw.events = dw.events[len(dw.events)-100:]
	}

	for ch := range dw.clients {
		select {
		case ch <- event:
		default:
			// Client buffer full, skip
		}
	}
}

// GetRecentEvents returns recent events for initial page load
func (dw *DiaryWatcher) GetRecentEvents(limit int) []DiaryEvent {
	dw.mu.RLock()
	defer dw.mu.RUnlock()

	if limit > len(dw.events) {
		limit = len(dw.events)
	}

	// Return most recent events
	result := make([]DiaryEvent, limit)
	copy(result, dw.events[len(dw.events)-limit:])
	return result
}

// findDiaryFiles recursively finds all diary.md files
func (dw *DiaryWatcher) findDiaryFiles() ([]string, error) {
	var files []string

	err := filepath.WalkDir(dw.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !d.IsDir() && strings.ToLower(d.Name()) == "diary.md" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// watchDirectory adds a directory and its subdirectories to the watcher
func (dw *DiaryWatcher) watchDirectory(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if err := dw.watcher.Add(path); err != nil {
				log.Printf("Warning: could not watch %s: %v", path, err)
			}
		}
		return nil
	})
}

// Start begins watching for diary changes
func (dw *DiaryWatcher) Start() error {
	// Initial scan for existing diary files
	files, err := dw.findDiaryFiles()
	if err != nil {
		return err
	}

	// Initialize section tracking for existing files
	for _, f := range files {
		sections, err := parseSections(f)
		if err == nil {
			dw.fileSections[f] = sections
		}
	}

	// Watch the root directory recursively
	if err := dw.watchDirectory(dw.rootDir); err != nil {
		return err
	}

	log.Printf("Watching %d diary files in %s", len(files), dw.rootDir)

	go dw.eventLoop()
	return nil
}

func (dw *DiaryWatcher) eventLoop() {
	// Debounce map for rapid file changes
	pending := make(map[string]time.Time)
	debounceDelay := 500 * time.Millisecond

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-dw.watcher.Events:
			if !ok {
				return
			}

			// Check if this is a diary.md file
			if strings.ToLower(filepath.Base(event.Name)) != "diary.md" {
				// Check if it's a new directory
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						dw.watcher.Add(event.Name)
					}
				}
				continue
			}

			pending[event.Name] = time.Now()

		case err, ok := <-dw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)

		case <-ticker.C:
			now := time.Now()
			for path, when := range pending {
				if now.Sub(when) >= debounceDelay {
					delete(pending, path)
					dw.processFileChange(path)
				}
			}
		}
	}
}

func (dw *DiaryWatcher) processFileChange(filePath string) {
	newSections, err := parseSections(filePath)
	if err != nil {
		log.Printf("Error parsing %s: %v", filePath, err)
		return
	}

	dw.mu.Lock()
	oldSections := dw.fileSections[filePath]
	dw.fileSections[filePath] = newSections
	dw.mu.Unlock()

	// Get ticket metadata
	ticketDir := filepath.Dir(filePath)
	// Walk up to find the ticket directory (contains --)
	for !strings.Contains(filepath.Base(ticketDir), "--") && ticketDir != dw.rootDir {
		ticketDir = filepath.Dir(ticketDir)
	}
	meta := parseTicketMeta(ticketDir)

	// Find new sections
	added := getNewSections(oldSections, newSections)

	if len(added) > 0 {
		for _, section := range added {
			content := getSectionContent(filePath, section)
			event := DiaryEvent{
				Timestamp:   time.Now(),
				FilePath:    filePath,
				TicketID:    meta.TicketID,
				Title:       meta.Title,
				Section:     section,
				Preview:     content.Preview,
				FullContent: content.Full,
				EventType:   "new_section",
			}
			dw.broadcast(event)
			log.Printf("New section in %s: %s", meta.TicketID, section)
		}
	} else if len(oldSections) == 0 && len(newSections) > 0 {
		// File was created
		msg := fmt.Sprintf("Diary created with %d sections", len(newSections))
		event := DiaryEvent{
			Timestamp:   time.Now(),
			FilePath:    filePath,
			TicketID:    meta.TicketID,
			Title:       meta.Title,
			Section:     "File Created",
			Preview:     msg,
			FullContent: msg,
			EventType:   "created",
		}
		dw.broadcast(event)
	}
}

// GetAllDiaries returns all diary files with their metadata, sorted by last modified
func (dw *DiaryWatcher) GetAllDiaries() ([]DiaryInfo, error) {
	files, err := dw.findDiaryFiles()
	if err != nil {
		return nil, err
	}

	var diaries []DiaryInfo
	for _, filePath := range files {
		// Get file info
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		// Get ticket metadata
		ticketDir := filepath.Dir(filePath)
		for !strings.Contains(filepath.Base(ticketDir), "--") && ticketDir != dw.rootDir {
			ticketDir = filepath.Dir(ticketDir)
		}
		meta := parseTicketMeta(ticketDir)

		// Get section count
		sections, err := parseSections(filePath)
		sectionCount := len(sections)
		if err != nil {
			sectionCount = 0
		}

		diaries = append(diaries, DiaryInfo{
			FilePath:     filePath,
			TicketID:     meta.TicketID,
			Title:        meta.Title,
			LastModified: info.ModTime(),
			SectionCount: sectionCount,
		})
	}

	// Sort by last modified (most recent first)
	for i := 0; i < len(diaries)-1; i++ {
		for j := i + 1; j < len(diaries); j++ {
			if diaries[i].LastModified.Before(diaries[j].LastModified) {
				diaries[i], diaries[j] = diaries[j], diaries[i]
			}
		}
	}

	return diaries, nil
}

// Close stops the watcher
func (dw *DiaryWatcher) Close() error {
	return dw.watcher.Close()
}

// HTTP Handlers

type Server struct {
	watcher   *DiaryWatcher
	templates *template.Template
}

func NewServer(watcher *DiaryWatcher) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		watcher:   watcher,
		templates: tmpl,
	}, nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Title  string
		Events []DiaryEvent
	}{
		Title:  "Diary Tail - Event Viewer",
		Events: s.watcher.GetRecentEvents(50),
	}

	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDiaries(w http.ResponseWriter, r *http.Request) {
	diaries, err := s.watcher.GetAllDiaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Title   string
		Diaries []DiaryInfo
	}{
		Title:   "Diary Tail - All Diaries",
		Diaries: diaries,
	}

	if err := s.templates.ExecuteTemplate(w, "diaries.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleAPIDiaries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	diaries, err := s.watcher.GetAllDiaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(diaries)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to events
	eventCh := s.watcher.Subscribe()
	defer s.watcher.Unsubscribe(eventCh)

	// Send keepalive
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-eventCh:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	events := s.watcher.GetRecentEvents(50)
	json.NewEncoder(w).Encode(events)
}

// handleDiary serves the full diary content for a given file path
func (s *Server) handleDiary(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		http.Error(w, "file parameter required", http.StatusBadRequest)
		return
	}

	// Security: ensure the file is within the watched directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	absRoot, _ := filepath.Abs(s.watcher.rootDir)
	if !strings.HasPrefix(absPath, absRoot) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Get ticket metadata
	ticketDir := filepath.Dir(filePath)
	for !strings.Contains(filepath.Base(ticketDir), "--") && ticketDir != s.watcher.rootDir {
		ticketDir = filepath.Dir(ticketDir)
	}
	meta := parseTicketMeta(ticketDir)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"content":  string(content),
		"filePath": filePath,
		"ticketId": meta.TicketID,
		"title":    meta.Title,
	})
}

func main() {
	port := flag.Int("port", 8765, "HTTP server port")
	dir := flag.String("dir", ".", "Root directory to watch for diary.md files")
	flag.Parse()

	// Resolve to absolute path
	absDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("Invalid directory: %v", err)
	}

	log.Printf("Starting Diary Tail App...")
	log.Printf("Watching directory: %s", absDir)

	watcher, err := NewDiaryWatcher(absDir)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Start(); err != nil {
		log.Fatalf("Failed to start watcher: %v", err)
	}

	server, err := NewServer(watcher)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Serve static files
	staticContent, _ := fs.Sub(staticFS, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	// Routes
	http.HandleFunc("/", server.handleIndex)
	http.HandleFunc("/diaries", server.handleDiaries)
	http.HandleFunc("/events", server.handleEvents)
	http.HandleFunc("/api/events", server.handleAPI)
	http.HandleFunc("/api/diary", server.handleDiary)
	http.HandleFunc("/api/diaries", server.handleAPIDiaries)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Server listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

