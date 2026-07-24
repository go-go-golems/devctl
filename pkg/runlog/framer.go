package runlog

import "bytes"

const DefaultMaxRecordBytes = 1 << 20

type Frame struct {
	Text    string
	Partial bool
}

type Framer struct {
	maxRecordBytes int
	buffer         []byte
}

func NewFramer(maxRecordBytes int) *Framer {
	if maxRecordBytes <= 0 {
		maxRecordBytes = DefaultMaxRecordBytes
	}
	return &Framer{maxRecordBytes: maxRecordBytes}
}

func (f *Framer) Write(data []byte) []Frame {
	f.buffer = append(f.buffer, data...)
	frames := make([]Frame, 0)
	for {
		newline := bytes.IndexByte(f.buffer, '\n')
		if newline >= 0 {
			line := f.buffer[:newline]
			for len(line) > f.maxRecordBytes {
				frames = append(frames, Frame{
					Text:    string(line[:f.maxRecordBytes]),
					Partial: true,
				})
				line = line[f.maxRecordBytes:]
			}
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			frames = append(frames, Frame{Text: string(line)})
			f.buffer = f.buffer[newline+1:]
			continue
		}
		if len(f.buffer) > f.maxRecordBytes {
			frames = append(frames, Frame{
				Text:    string(f.buffer[:f.maxRecordBytes]),
				Partial: true,
			})
			f.buffer = f.buffer[f.maxRecordBytes:]
			continue
		}
		return frames
	}
}

func (f *Framer) Close() []Frame {
	if len(f.buffer) == 0 {
		return nil
	}
	frame := Frame{Text: string(f.buffer), Partial: true}
	f.buffer = nil
	return []Frame{frame}
}
