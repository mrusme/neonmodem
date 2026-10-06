package credential

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxOutput     = 64 << 10
	stderrTail    = 1 << 10
	stderrLineMax = 200
)

var (
	ErrNoOutput      = errors.New("printed an empty first line")
	ErrOutputTooLong = errors.New("printed a first line longer than 64 KiB")
)

var escapeSequence = regexp.MustCompile(`\x1b(\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|.)`)

type cappedBuffer struct {
	data     []byte
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	room := max(b.limit-len(b.data), 0)
	if n > room {
		b.overflow = true
		p = p[:room]
	}
	b.data = append(b.data, p...)
	return n, nil
}

func (b *cappedBuffer) firstLine() (string, error) {
	line, _, found := bytes.Cut(b.data, []byte("\n"))
	if !found && b.overflow {
		return "", ErrOutputTooLong
	}
	line = bytes.TrimSuffix(line, []byte("\r"))
	if len(line) == 0 {
		return "", ErrNoOutput
	}
	return string(line), nil
}

type tailBuffer struct {
	data  []byte
	limit int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	if extra := len(b.data) - b.limit; extra > 0 {
		b.data = append(b.data[:0], b.data[extra:]...)
	}
	return len(p), nil
}

func (b *tailBuffer) lastLine() string {
	text := escapeSequence.ReplaceAllString(string(b.data), "")
	text = strings.TrimRight(text, " \t\r\n")
	if i := strings.LastIndexAny(text, "\r\n"); i >= 0 {
		text = text[i+1:]
	}

	text = strings.TrimSpace(strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case !unicode.IsPrint(r):
			return -1
		}
		return r
	}, text))

	if utf8.RuneCountInString(text) > stderrLineMax {
		text = string([]rune(text)[:stderrLineMax]) + "..."
	}
	return text
}
