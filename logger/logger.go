package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// Global pool of bytes.Buffer to eliminate allocations in Handle
var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// PrettyHandler is a custom slog.Handler that outputs logs with colored formatting.
type PrettyHandler struct {
	writer      io.Writer
	level       slog.Level
	color       bool
	json        bool
	jsonHandler slog.Handler
	attrs       []slog.Attr
	groups      []string
	groupPrefix string // Pre-computed strings.Join(groups, ".")
	mu          sync.Mutex
}

// Enabled reports whether the handler handles records at the given level.
func (h *PrettyHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

// Handle formats and writes the record.
func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.json {
		return h.jsonHandler.Handle(ctx, r)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	// 1. Format Time (allocation-free)
	if h.color {
		buf.WriteString(colorGray)
		appendTime(buf, r.Time)
		buf.WriteString(colorReset + " ")
	} else {
		appendTime(buf, r.Time)
		buf.WriteByte(' ')
	}

	// 2. Format Level
	levelStr := r.Level.String()
	if h.color {
		var levelColor string
		switch r.Level {
		case slog.LevelDebug:
			levelColor = colorCyan
		case slog.LevelInfo:
			levelColor = colorGreen
		case slog.LevelWarn:
			levelColor = colorYellow
		case slog.LevelError:
			levelColor = colorRed + colorBold
		default:
			levelColor = colorBlue
		}
		buf.WriteString(levelColor + "[" + levelStr + "]" + colorReset + " ")
	} else {
		buf.WriteString("[" + levelStr + "] ")
	}

	// 3. Format Message
	if h.color {
		buf.WriteString(colorBold + r.Message + colorReset)
	} else {
		buf.WriteString(r.Message)
	}

	// Recursive writer for attributes (allocation-free)
	var writeAttr func(prefix string, a slog.Attr)
	writeAttr = func(prefix string, a slog.Attr) {
		a.Value = a.Value.Resolve()
		if a.Equal(slog.Attr{}) {
			return
		}

		key := a.Key
		if prefix != "" {
			// Write key with prefix without allocating a new string
			if h.color {
				buf.WriteString(" " + colorGray + prefix + "." + key + "=" + colorReset)
			} else {
				buf.WriteString(" " + prefix + "." + key + "=")
			}
		} else {
			if h.color {
				buf.WriteString(" " + colorGray + key + "=" + colorReset)
			} else {
				buf.WriteString(" " + key + "=")
			}
		}

		if a.Value.Kind() == slog.KindGroup {
			groupAttrs := a.Value.Group()
			// We nested under the current prefix + key
			var nextPrefix string
			if prefix != "" {
				nextPrefix = prefix + "." + key
			} else {
				nextPrefix = key
			}
			for _, ga := range groupAttrs {
				writeAttr(nextPrefix, ga)
			}
			return
		}

		writeValue(buf, a.Value)
	}

	// Write accumulated handler attributes
	for _, attr := range h.attrs {
		writeAttr("", attr)
	}

	// Write record attributes under the current group prefix
	r.Attrs(func(attr slog.Attr) bool {
		writeAttr(h.groupPrefix, attr)
		return true
	})

	buf.WriteByte('\n')

	_, err := h.writer.Write(buf.Bytes())
	return err
}

// WithAttrs returns a new handler containing the given attributes.
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var delegate slog.Handler
	if h.json {
		delegate = h.jsonHandler.WithAttrs(attrs)
	}

	// Prefix the keys of the new attributes with the current group prefix
	var prefixedAttrs []slog.Attr
	if h.groupPrefix != "" {
		prefixedAttrs = make([]slog.Attr, len(attrs))
		for i, a := range attrs {
			prefixedAttrs[i] = slog.Attr{Key: h.groupPrefix + "." + a.Key, Value: a.Value}
		}
	} else {
		prefixedAttrs = attrs
	}

	newAttrs := make([]slog.Attr, len(h.attrs)+len(prefixedAttrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], prefixedAttrs)

	return &PrettyHandler{
		writer:      h.writer,
		level:       h.level,
		color:       h.color,
		json:        h.json,
		jsonHandler: delegate,
		attrs:       newAttrs,
		groups:      h.groups,
		groupPrefix: h.groupPrefix,
	}
}

// WithGroup returns a new handler with the given group name.
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	var delegate slog.Handler
	if h.json {
		delegate = h.jsonHandler.WithGroup(name)
	}

	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	groupPrefix := strings.Join(newGroups, ".")

	return &PrettyHandler{
		writer:      h.writer,
		level:       h.level,
		color:       h.color,
		json:        h.json,
		jsonHandler: delegate,
		attrs:       h.attrs,
		groups:      newGroups,
		groupPrefix: groupPrefix,
	}
}

// writeValue formats slog.Value directly to the buffer without allocations.
func writeValue(buf *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		str := v.String()
		if strings.ContainsAny(str, " \t\n\"\\") {
			buf.WriteByte('"')
			for i := 0; i < len(str); i++ {
				c := str[i]
				if c == '"' || c == '\\' {
					buf.WriteByte('\\')
				}
				buf.WriteByte(c)
			}
			buf.WriteByte('"')
		} else {
			buf.WriteString(str)
		}
	case slog.KindInt64:
		appendInt64(buf, v.Int64())
	case slog.KindUint64:
		appendUint64(buf, v.Uint64())
	case slog.KindFloat64:
		// Fallback for floats
		fmt.Fprintf(buf, "%g", v.Float64())
	case slog.KindBool:
		if v.Bool() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case slog.KindTime:
		appendTime(buf, v.Time())
	default:
		buf.WriteString(v.String())
	}
}

// appendTime formats a time.Time directly into the buffer without allocations.
func appendTime(buf *bytes.Buffer, t time.Time) {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	ms := t.Nanosecond() / int(time.Millisecond)

	appendInt(buf, year, 4)
	buf.WriteByte('-')
	appendInt(buf, int(month), 2)
	buf.WriteByte('-')
	appendInt(buf, day, 2)
	buf.WriteByte(' ')
	appendInt(buf, hour, 2)
	buf.WriteByte(':')
	appendInt(buf, min, 2)
	buf.WriteByte(':')
	appendInt(buf, sec, 2)
	buf.WriteByte('.')
	appendInt(buf, ms, 3)
}

// appendInt appends an integer with zero-padding to the buffer without allocations.
func appendInt(buf *bytes.Buffer, val int, width int) {
	var tmp [10]byte
	i := len(tmp)
	for val > 0 || width > 0 {
		i--
		tmp[i] = byte('0' + val%10)
		val /= 10
		width--
	}
	buf.Write(tmp[i:])
}

// appendInt64 appends an int64 to the buffer without allocations.
func appendInt64(buf *bytes.Buffer, val int64) {
	if val == 0 {
		buf.WriteByte('0')
		return
	}
	if val < 0 {
		buf.WriteByte('-')
		val = -val
	}
	var tmp [20]byte
	i := len(tmp)
	for val > 0 {
		i--
		tmp[i] = byte('0' + val%10)
		val /= 10
	}
	buf.Write(tmp[i:])
}

// appendUint64 appends a uint64 to the buffer without allocations.
func appendUint64(buf *bytes.Buffer, val uint64) {
	if val == 0 {
		buf.WriteByte('0')
		return
	}
	var tmp [20]byte
	i := len(tmp)
	for val > 0 {
		i--
		tmp[i] = byte('0' + val%10)
		val /= 10
	}
	buf.Write(tmp[i:])
}

// LoggerBuilder is a fluent configuration builder for slog.Logger.
type LoggerBuilder struct {
	level  slog.Level
	writer io.Writer
	color  bool
	json   bool
	err    error
}

// New creates a new fluent LoggerBuilder with defaults.
func New() *LoggerBuilder {
	return &LoggerBuilder{
		level:  slog.LevelInfo,
		writer: os.Stdout,
		color:  true,
		json:   false,
	}
}

// WithLevel sets the minimum logging level.
func (b *LoggerBuilder) WithLevel(l slog.Level) *LoggerBuilder {
	if b.err != nil {
		return b
	}
	b.level = l
	return b
}

// WithWriter sets the output writer.
func (b *LoggerBuilder) WithWriter(w io.Writer) *LoggerBuilder {
	if b.err != nil {
		return b
	}
	if w == nil {
		b.err = fmt.Errorf("writer cannot be nil")
		return b
	}
	b.writer = w
	return b
}

// WithColor enables or disables ANSI terminal colors.
func (b *LoggerBuilder) WithColor(c bool) *LoggerBuilder {
	if b.err != nil {
		return b
	}
	b.color = c
	return b
}

// WithJSON enables or disables JSON output format.
func (b *LoggerBuilder) WithJSON(j bool) *LoggerBuilder {
	if b.err != nil {
		return b
	}
	b.json = j
	return b
}

// Error returns any configuration error.
func (b *LoggerBuilder) Error() error {
	return b.err
}

// Build compiles the handler and returns a configured *slog.Logger.
func (b *LoggerBuilder) Build() *slog.Logger {
	var jsonHandler slog.Handler
	if b.json {
		jsonHandler = slog.NewJSONHandler(b.writer, &slog.HandlerOptions{
			Level: b.level,
		})
	}

	handler := &PrettyHandler{
		writer:      b.writer,
		level:       b.level,
		color:       b.color && !b.json,
		json:        b.json,
		jsonHandler: jsonHandler,
	}

	return slog.New(handler)
}
