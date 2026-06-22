// Package xlogger wraps zerolog with a small typed-field helper API.
package xlogger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config controls logger output.
type Config struct {
	Level      string `mapstructure:"level"`       // debug | info | warn | error
	Format     string `mapstructure:"format"`      // console | json
	Output     string `mapstructure:"output"`      // stdout | stderr | /path/to/file
	TimeFormat string `mapstructure:"time_format"` // e.g. 2006-01-02T15:04:05.000Z07:00
}

// Logger is the application logger.
type Logger struct{ z zerolog.Logger }

// New builds a Logger from cfg.
func New(cfg *Config) (*Logger, error) {
	level, err := zerolog.ParseLevel(strings.ToLower(orDefault(cfg.Level, "info")))
	if err != nil {
		return nil, err
	}
	var w io.Writer
	switch strings.ToLower(orDefault(cfg.Output, "stdout")) {
	case "stderr":
		w = os.Stderr
	case "stdout", "":
		w = os.Stdout
	default:
		f, ferr := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if ferr != nil {
			return nil, ferr
		}
		w = f
	}
	tf := orDefault(cfg.TimeFormat, time.RFC3339)
	if strings.EqualFold(cfg.Format, "console") {
		w = zerolog.ConsoleWriter{Out: w, TimeFormat: tf}
	}
	zerolog.TimeFieldFormat = tf
	z := zerolog.New(w).Level(level).With().Timestamp().Logger()
	return &Logger{z: z}, nil
}

// Nop returns a no-op logger (useful in tests).
func Nop() *Logger { return &Logger{z: zerolog.Nop()} }

// NewWithWriter builds a Logger that writes JSON to w at the given level.
// Primarily for tests that need to assert on emitted log lines.
func NewWithWriter(w io.Writer, level string) *Logger {
	lvl, err := zerolog.ParseLevel(strings.ToLower(orDefault(level, "info")))
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	z := zerolog.New(w).Level(lvl).With().Timestamp().Logger()
	return &Logger{z: z}
}

// Field is a structured log field.
type Field struct {
	key string
	val any
}

func String(k, v string) Field            { return Field{k, v} }
func Int(k string, v int) Field           { return Field{k, v} }
func Int64(k string, v int64) Field       { return Field{k, v} }
func Bool(k string, v bool) Field         { return Field{k, v} }
func Any(k string, v any) Field           { return Field{k, v} }
func Dur(k string, v time.Duration) Field { return Field{k, v.String()} }
func Err(err error) Field                 { return Field{"error", errString(err)} }

func errString(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}

func (l *Logger) emit(ev *zerolog.Event, msg string, fields []Field) {
	for _, f := range fields {
		ev = ev.Interface(f.key, f.val)
	}
	ev.Msg(msg)
}

func (l *Logger) Debug(msg string, fields ...Field) { l.emit(l.z.Debug(), msg, fields) }
func (l *Logger) Info(msg string, fields ...Field)  { l.emit(l.z.Info(), msg, fields) }
func (l *Logger) Warn(msg string, fields ...Field)  { l.emit(l.z.Warn(), msg, fields) }
func (l *Logger) Error(msg string, fields ...Field) { l.emit(l.z.Error(), msg, fields) }
func (l *Logger) Fatal(msg string, fields ...Field) { l.emit(l.z.Fatal(), msg, fields) }

// Z exposes the underlying zerolog.Logger.
func (l *Logger) Z() *zerolog.Logger { return &l.z }

// With returns a child logger with persistent fields.
func (l *Logger) With(fields ...Field) *Logger {
	c := l.z.With()
	for _, f := range fields {
		c = c.Interface(f.key, f.val)
	}
	return &Logger{z: c.Logger()}
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
