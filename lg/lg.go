package lg

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel log level
type LogLevel int

const (
	DEBUG = LogLevel(1)
	INFO  = LogLevel(2)
	WARN  = LogLevel(3)
	ERROR = LogLevel(4)
	FATAL = LogLevel(5)
)

type AppLogFunc func(lvl LogLevel, f string, args ...interface{})

// Logger
type Logger interface {
	Logf(msgLevel LogLevel, format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Fatalf(format string, v ...interface{})
}

type logger struct {
	goLogger   *log.Logger
	logLevel   LogLevel
	exitFunc   func()
	debugLabel string
	infoLabel  string
	warnLabel  string
	errorLabel string
	fatalLabel string
}

// Generate the pid prefix string
func pidPrefix() string {
	return fmt.Sprintf("[%d] ", os.Getpid())
}

func setLabelFormats(l *logger, colors bool) {
	if colors {
		colorFormat := "[\x1b[%dm%s\x1b[0m] "
		l.debugLabel = fmt.Sprintf(colorFormat, 36, "DBG")
		l.infoLabel = fmt.Sprintf(colorFormat, 32, "INF")
		l.warnLabel = fmt.Sprintf(colorFormat, 33, "WRN")
		l.errorLabel = fmt.Sprintf(colorFormat, 31, "ERR")
		l.fatalLabel = fmt.Sprintf(colorFormat, 35, "FTL")
	} else {
		l.debugLabel = "[DBG] "
		l.infoLabel = "[INF] "
		l.warnLabel = "[WRN] "
		l.errorLabel = "[ERR] "
		l.fatalLabel = "[FTL] "
	}
}

// NewStdLogger creates a logger with output directed to Stderr
func NewStdLogger(prefix string, time, colors, pid bool, logLevel LogLevel, exitFunc func()) Logger {
	flags := 0
	if time {
		flags = log.LstdFlags | log.Lmicroseconds
	}

	pre := prefix
	if pid {
		pre += pidPrefix()
	}

	l := &logger{
		goLogger: log.New(os.Stderr, pre, flags),
		logLevel: logLevel,
		exitFunc: exitFunc,
	}

	setLabelFormats(l, colors)

	return l
}

func NewCommonStdLoggerWithLevelStr(prefix, levelStr string, exitFunc func()) (Logger, error) {
	logLevel, err := ParseLogLevel(levelStr)
	if err != nil {
		return nil, err
	}
	colors := true
	stat, err := os.Stderr.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
		colors = false
	}
	return NewStdLogger(prefix, true, colors, (prefix == ""), logLevel, exitFunc), nil
}

// ParseLogLevel parse a levvel str to LogLevel
func ParseLogLevel(levelstr string) (LogLevel, error) {
	lvl := INFO

	switch strings.ToLower(levelstr) {
	case "debug":
		lvl = DEBUG
	case "info":
		lvl = INFO
	case "warn":
		lvl = WARN
	case "error":
		lvl = ERROR
	case "fatal":
		lvl = FATAL
	default:
		return lvl, fmt.Errorf("invalid log-level '%s'", levelstr)
	}

	return lvl, nil
}

// Logf printf log with level and format
func (l *logger) Logf(msgLevel LogLevel, format string, v ...interface{}) {
	if l.logLevel > msgLevel {
		return
	}

	label := ""

	switch msgLevel {
	case DEBUG:
		label = l.debugLabel
	case INFO:
		label = l.infoLabel
	case WARN:
		label = l.warnLabel
	case ERROR:
		label = l.errorLabel
	case FATAL:
		label = l.fatalLabel
	}

	l.goLogger.Printf(label+format, v...)
	// fmt.Printf(label+format+"\n", v...)
}

// Debugf logs a debug statement
func (l *logger) Debugf(format string, v ...interface{}) {
	l.Logf(DEBUG, format, v...)
}

// Infof logs a info statement
func (l *logger) Infof(format string, v ...interface{}) {
	l.Logf(INFO, format, v...)
}

// Warnf logs a warn statement
func (l *logger) Warnf(format string, v ...interface{}) {
	l.Logf(WARN, format, v...)
}

// Errorf logs a error statement
func (l *logger) Errorf(format string, v ...interface{}) {
	l.Logf(ERROR, format, v...)
}

// Fatalf logs a fatal statement
func (l *logger) Fatalf(format string, v ...interface{}) {
	l.Logf(FATAL, format, v...)
	if l.exitFunc != nil {
		l.exitFunc()
		return
	}
	os.Exit(1)
}
