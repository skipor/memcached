// Package log contains simple leveled logging implementation on top of stdlib logger.
// NOTE: without "only stdlib" constraint I would use github.com/uber-go/zap for logging.
package log

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

// Logger interface is subset of github.com/uber-common/bark.Logger methods.
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
}

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	}
	panic("unexpected level: " + strconv.Itoa(int(l)))
}

var stringToLevel = func() map[string]Level {
	var levels = []Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, FatalLevel}
	res := make(map[string]Level, len(levels))
	for _, l := range levels {
		res[l.String()] = l
	}
	return res
}()

func LevelFromString(s string) (Level, error) {
	var err error
	l, ok := stringToLevel[s]
	if !ok {
		err = errors.New("invalid level " + s)
	}
	return l, err
}

const stdLoggerFlags = log.LstdFlags | log.Lmicroseconds | log.Lshortfile

func NewLogger(l Level, w io.Writer) Logger {
	return NewLoggerSink(l, &stdSink{log.New(w, "", stdLoggerFlags)})
}

func NewLoggerSink(l Level, s Sink) Logger {
	return &logger{
		sink:  s,
		level: l,
	}
}

// logger is primitive stdlib log.Logger wrapper for more common interface.
type logger struct {
	sink  Sink
	level Level
	depth int
}

func (l *logger) Debug(args ...interface{})                 { l.log(DebugLevel, args...) }
func (l *logger) Debugf(format string, args ...interface{}) { l.logf(DebugLevel, format, args...) }
func (l *logger) Info(args ...interface{})                  { l.log(InfoLevel, args...) }
func (l *logger) Infof(format string, args ...interface{})  { l.logf(InfoLevel, format, args...) }
func (l *logger) Warn(args ...interface{})                  { l.log(WarnLevel, args...) }
func (l *logger) Warnf(format string, args ...interface{})  { l.logf(WarnLevel, format, args...) }
func (l *logger) Error(args ...interface{})                 { l.log(ErrorLevel, args...) }
func (l *logger) Errorf(format string, args ...interface{}) { l.logf(ErrorLevel, format, args...) }
func (l *logger) Panic(args ...interface{}) {
	msg := fmt.Sprint(args...)
	l.log(ErrorLevel, msg)
	panic(msg)
}
func (l *logger) Panicf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.log(ErrorLevel, msg)
	panic(msg)
}
func (l *logger) Fatal(args ...interface{}) {
	l.log(FatalLevel, args...)
	os.Exit(1)
}
func (l *logger) Fatalf(format string, args ...interface{}) {
	l.logf(FatalLevel, format, args...)
	os.Exit(1)
}

type Sink interface {
	Output(callDepth int, l Level, msg string)
}

type stdSink struct {
	std *log.Logger
}

func (s *stdSink) Output(callDepth int, l Level, msg string) {
	s.std.Output(callDepth+1, l.String()+": "+msg)
}

const initialLoggerCallDepth = 3

func (l *logger) log(level Level, args ...interface{}) {
	if level >= l.level {
		l.sink.Output(l.depth+initialLoggerCallDepth, level, fmt.Sprint(args...))
	}
}

func (l *logger) logf(level Level, format string, args ...interface{}) {

	if level >= l.level {
		l.sink.Output(l.depth+initialLoggerCallDepth, level, fmt.Sprintf(format, args...))
	}
}
