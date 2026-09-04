package core

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var tokenPattern = regexp.MustCompile(`(?i)([?&]token=)[^\s&"'<>]+`)
var secretPattern = regexp.MustCompile(`(?i)((?:api[_-]?key|authorization|cookie|password|secret)["']?\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\r\n,}]+)`)
var ansiPattern = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

func Redact(s string) string {
	s = ansiPattern.ReplaceAllString(s, "")
	s = tokenPattern.ReplaceAllString(s, "${1}<REDACTED>")
	return secretPattern.ReplaceAllString(s, "${1}<REDACTED>")
}

type LogLine struct {
	Time string `json:"time"`
	Text string `json:"text"`
}
type Log struct {
	mu    sync.Mutex
	lines []LogLine
	path  string
}

func (l *Log) Add(s string) {
	s = strings.TrimSpace(Redact(s))
	if s == "" {
		return
	}
	if len(s) > 8192 {
		s = s[:8192] + "…"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, LogLine{time.Now().Format("15:04:05"), s})
	// Only redacted text reaches disk. Bound diagnostic storage to two files;
	// failures to write logs never prevent stopping the managed process.
	if l.path != "" {
		if info, e := os.Stat(l.path); e == nil && info.Size() > 5<<20 {
			_ = os.Remove(l.path + ".previous")
			_ = os.Rename(l.path, l.path+".previous")
		}
		if f, e := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); e == nil {
			_, _ = f.WriteString(time.Now().Format(time.RFC3339) + " " + s + "\n")
			_ = f.Close()
		}
	}
	if len(l.lines) > 500 {
		l.lines = append([]LogLine(nil), l.lines[len(l.lines)-500:]...)
	}
}
func (l *Log) Lines() []LogLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]LogLine{}, l.lines...)
}

// ReadLines reassembles split stdout chunks before parsing authentication URLs.
// Oversized output is bounded: a noisy plugin must not exhaust the desktop RAM.
func ReadLines(r io.Reader, consume func(string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		consume(scanner.Text())
	}
	return scanner.Err()
}
