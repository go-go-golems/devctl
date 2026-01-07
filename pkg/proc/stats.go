// Package proc provides utilities for reading process statistics from /proc.
package proc

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Stats contains process statistics read from /proc.
type Stats struct {
	PID        int     `json:"pid"`
	CPUPercent float64 `json:"cpu_percent"` // CPU usage as percentage (0-100+)
	MemoryMB   int64   `json:"memory_mb"`   // Resident memory in megabytes
	MemoryRSS  int64   `json:"memory_rss"`  // Resident set size in bytes
	VirtualMB  int64   `json:"virtual_mb"`  // Virtual memory in megabytes
	State      string  `json:"state"`       // Process state (R, S, D, Z, T, etc.)
	Threads    int     `json:"threads"`     // Number of threads
	StartTime  int64   `json:"start_time"`  // Process start time in jiffies since boot
}

// procStat holds parsed /proc/[pid]/stat fields needed for CPU calculation.
type procStat struct {
	utime     uint64 // User mode jiffies
	stime     uint64 // Kernel mode jiffies
	startTime uint64 // Start time in jiffies since boot
	state     byte   // Process state
	threads   int    // Number of threads
	vsize     uint64 // Virtual memory size in bytes
	rss       int64  // Resident set size in pages
}

// cpuSnapshot stores timing for CPU percentage calculation.
type cpuSnapshot struct {
	pid       int
	utime     uint64
	stime     uint64
	timestamp time.Time
}

// CPUTracker tracks CPU usage across multiple samples.
type CPUTracker struct {
	snapshots map[int]cpuSnapshot
}

// NewCPUTracker creates a new CPU usage tracker.
func NewCPUTracker() *CPUTracker {
	return &CPUTracker{
		snapshots: make(map[int]cpuSnapshot),
	}
}

// ReadStats reads process statistics for a single PID.
// If tracker is non-nil, it's used to calculate CPU percentage between calls.
func ReadStats(pid int, tracker *CPUTracker) (*Stats, error) {
	if pid <= 0 {
		return nil, errors.New("invalid PID")
	}

	ps, err := readProcStat(pid)
	if err != nil {
		return nil, errors.Wrap(err, "read /proc/stat")
	}

	pageSize := int64(os.Getpagesize())
	memRSS := ps.rss * pageSize
	memMB := memRSS / (1024 * 1024)
	virtualMB := int64(ps.vsize) / (1024 * 1024)

	stats := &Stats{
		PID:       pid,
		MemoryRSS: memRSS,
		MemoryMB:  memMB,
		VirtualMB: virtualMB,
		State:     string(ps.state),
		Threads:   ps.threads,
		StartTime: int64(ps.startTime),
	}

	// Calculate CPU percentage if we have a tracker
	if tracker != nil {
		now := time.Now()
		totalTime := ps.utime + ps.stime

		if prev, ok := tracker.snapshots[pid]; ok {
			elapsed := now.Sub(prev.timestamp).Seconds()
			if elapsed > 0 {
				prevTotal := prev.utime + prev.stime
				cpuDelta := float64(totalTime - prevTotal)
				// Convert jiffies to seconds (assuming 100 Hz, standard on Linux)
				cpuSeconds := cpuDelta / 100.0
				stats.CPUPercent = (cpuSeconds / elapsed) * 100.0
			}
		}

		tracker.snapshots[pid] = cpuSnapshot{
			pid:       pid,
			utime:     ps.utime,
			stime:     ps.stime,
			timestamp: now,
		}
	}

	return stats, nil
}

// ReadAllStats reads statistics for multiple PIDs.
func ReadAllStats(pids []int, tracker *CPUTracker) (map[int]*Stats, error) {
	result := make(map[int]*Stats)

	for _, pid := range pids {
		stats, err := ReadStats(pid, tracker)
		if err != nil {
			// Process may have exited, skip it
			continue
		}
		result[pid] = stats
	}

	return result, nil
}

// CleanupStale removes snapshots for PIDs no longer in the provided list.
func (t *CPUTracker) CleanupStale(activePIDs []int) {
	active := make(map[int]bool)
	for _, pid := range activePIDs {
		active[pid] = true
	}

	for pid := range t.snapshots {
		if !active[pid] {
			delete(t.snapshots, pid)
		}
	}
}

// readProcStat parses /proc/[pid]/stat file.
func readProcStat(pid int) (*procStat, error) {
	path := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read stat file")
	}

	// Format: pid (comm) state ppid pgrp session tty_nr tpgid flags minflt cminflt
	//         majflt cmajflt utime stime cutime cstime priority nice num_threads
	//         itrealvalue starttime vsize rss ...
	//
	// The comm field can contain spaces and parentheses, so we need to find
	// the last ')' to parse correctly.
	content := string(data)
	closeParen := strings.LastIndex(content, ")")
	if closeParen < 0 {
		return nil, errors.New("malformed stat file: no closing paren")
	}

	// Parse fields after the comm field
	rest := strings.TrimSpace(content[closeParen+1:])
	fields := strings.Fields(rest)

	// We need at least 22 fields after comm
	if len(fields) < 22 {
		return nil, fmt.Errorf("malformed stat file: expected 22+ fields, got %d", len(fields))
	}

	// Field indices (0-based after comm):
	// 0: state
	// 11: utime (index 13 in original, minus 2 for pid and comm)
	// 12: stime
	// 17: num_threads
	// 19: starttime
	// 20: vsize
	// 21: rss

	ps := &procStat{
		state: fields[0][0],
	}

	var parseErr error

	ps.utime, parseErr = strconv.ParseUint(fields[11], 10, 64)
	if parseErr != nil {
		return nil, errors.Wrap(parseErr, "parse utime")
	}

	ps.stime, parseErr = strconv.ParseUint(fields[12], 10, 64)
	if parseErr != nil {
		return nil, errors.Wrap(parseErr, "parse stime")
	}

	threads, parseErr := strconv.Atoi(fields[17])
	if parseErr != nil {
		return nil, errors.Wrap(parseErr, "parse num_threads")
	}
	ps.threads = threads

	ps.startTime, parseErr = strconv.ParseUint(fields[19], 10, 64)
	if parseErr != nil {
		return nil, errors.Wrap(parseErr, "parse starttime")
	}

	ps.vsize, parseErr = strconv.ParseUint(fields[20], 10, 64)
	if parseErr != nil {
		return nil, errors.Wrap(parseErr, "parse vsize")
	}

	rss, parseErr := strconv.ParseInt(fields[21], 10, 64)
	if parseErr != nil {
		return nil, errors.Wrap(parseErr, "parse rss")
	}
	ps.rss = rss

	return ps, nil
}

// GetBootTime returns the system boot time.
func GetBootTime() (time.Time, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return time.Time{}, errors.Wrap(err, "open /proc/stat")
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "btime ") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			btime, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return time.Time{}, errors.Wrap(err, "parse btime")
			}
			return time.Unix(btime, 0), nil
		}
	}

	return time.Time{}, errors.New("btime not found in /proc/stat")
}

// GetProcessStartTime returns when a process started based on /proc/[pid]/stat.
func GetProcessStartTime(pid int) (time.Time, error) {
	ps, err := readProcStat(pid)
	if err != nil {
		return time.Time{}, err
	}

	bootTime, err := GetBootTime()
	if err != nil {
		return time.Time{}, err
	}

	// startTime is in clock ticks since boot
	// Assuming 100 Hz (standard on Linux)
	startSeconds := int64(ps.startTime) / 100
	return bootTime.Add(time.Duration(startSeconds) * time.Second), nil
}

