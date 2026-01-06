package tui

import "time"

type ActionLog struct {
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}
