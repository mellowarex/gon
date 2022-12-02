package logs

import (
	"fmt"
	"log"
	"sync"
	"strings"
)

// Package logs provide a general log interface
// Usage:
//
//	log := NewLogger(10000) params stand for how many channel
//	log.SetLogger("console", "")
//
// Use it like this:
//
//	log.Trace("trace")
//	log.Info("info")
//	log.Warn("warning")
//	log.Debug("debug")
//	log.Critical("critical")

// RFC5424 log message levels.
const (
	LevelEmergency = iota
	LevelAlert
	LevelCritical
	LevelError
	LevelWarning
	LevelNotice
	LevelInformational
	LevelDebug
)

// log adapter
const (
	AdapterConsole   = "console"
	AdapterFile      = "file"
	AdapterMultiFile = "multifile"
	AdapterMail      = "smtp"
	AdapterConn      = "conn"
)

// Logger defines log provider behavior
type Logger interface {
	Init(config string) error
	WriteMsg(lm *LogMsg) error
	Destroy()
	Flush()
	SetFormatter(f LogFormatter)
}

type newLoggerFunc func() Logger
var adapters = make(map[string]newLoggerFunc)

var levelPrefix = [LevelDebug + 1]string{"[M]", "[A]", "[C]", "[E]", "[W]", "[N]", "[I]", "[D]"}

// levelLogLogger is defined to implement log.Logger
// the real log level will be LevelEmergency
const levelLoggerImpl = -1

// Register makes a log provide available by the provided name.
// If Register is called twice with the same name or if driver is nil,
// it panics.
func Register(name string, log newLoggerFunc) {
	if log == nil {
		panic("logs: Register provided is nil")
	}
	if _, ok := adapters[name]; ok {
		panic("logs: Register called twice for provider " + name)
	}
	adapters[name] = log
}

// gonLogger references the used application logger.
var gonLogger = NewLogger()

// don't forget to register the formatter by invoking RegisterFormatter
func SetGlobalFormatter(fmtter string) error {
	return gonLogger.setGlobalFormatter(fmtter)
}

// GetGonLogger returns default GonLogger
func GetGonLogger() *GonLogger {
	return gonLogger
}

var gonLoggerMap = struct {
	sync.RWMutex
	logs map[string]*log.Logger
}{
	logs: map[string]*log.Logger{},
}

// GetLogger returns the default gonLogger
func GetLogger(prefixes ...string) *log.Logger {
	prefix := append(prefixes, "")[0]
	if prefix != "" {
		prefix = fmt.Sprintf(`[%s] `, strings.ToUpper(prefix))
	}
	gonLoggerMap.RLock()
	l, ok := gonLoggerMap.logs[prefix]
	if ok {
		gonLoggerMap.RUnlock()
		return l
	}
	gonLoggerMap.RUnlock()
	gonLoggerMap.Lock()
	defer gonLoggerMap.Unlock()
	l, ok = gonLoggerMap.logs[prefix]
	if !ok {
		l = log.New(gonLogger, prefix, 0)
		gonLoggerMap.logs[prefix] = l
	}
	return l
}
// EnableFullFilePath enables full file path logging. Disabled by default
// e.g "/home/Documents/GitHub/beego/mainapp/" instead of "mainapp"
func EnableFullFilePath(b bool) {
	gonLogger.enableFullFilePath = b
}

// SetLogger sets a new logger.
func SetLogger(adapter string, config ...string) error {
	return gonLogger.SetLogger(adapter, config...)
}

// Reset will remove all the adapter
func Reset() {
	gonLogger.Reset()
}

// SetLogFuncCall set the CallDepth, default is 4
func SetLogFuncCall(b bool) {
	gonLogger.EnableFuncCallDepth(b)
	gonLogger.SetLogFuncCallDepth(3)
}

// Emergency logs a message at emergency level.
func Emergency(f interface{}, v ...interface{}) {
	gonLogger.Emergency(formatLog(f, v...))
}

// Alert logs a message at alert level.
func Alert(f interface{}, v ...interface{}) {
	gonLogger.Alert(formatLog(f, v...))
}

// Critical logs a message at critical level.
func Critical(f interface{}, v ...interface{}) {
	gonLogger.Critical(formatLog(f, v...))
}

// Error logs a message at error level.
func Error(f interface{}, v ...interface{}) {
	gonLogger.Error(formatLog(f, v...))
}

// Warning logs a message at warning level.
func Warning(f interface{}, v ...interface{}) {
	gonLogger.Warn(formatLog(f, v...))
}

// Warn compatibility alias for Warning()
func Warn(f interface{}, v ...interface{}) {
	gonLogger.Warn(formatLog(f, v...))
}

// Notice logs a message at notice level.
func Notice(f interface{}, v ...interface{}) {
	gonLogger.Notice(formatLog(f, v...))
}

// Informational logs a message at info level.
func Informational(f interface{}, v ...interface{}) {
	gonLogger.Info(formatLog(f, v...))
}

// Info compatibility alias for Warning()
func Info(f interface{}, v ...interface{}) {
	gonLogger.Info(formatLog(f, v...))
}

// Debug logs a message at debug level.
func Debug(f interface{}, v ...interface{}) {
	gonLogger.Debug(formatLog(f, v...))
}

// Trace logs a message at trace level.
// compatibility alias for Warning()
func Trace(f interface{}, v ...interface{}) {
	gonLogger.Trace(formatLog(f, v...))
}

func formatLog(f interface{}, v ...interface{}) string {
	var msg string
	switch f.(type) {
	case string:
		msg = f.(string)
		if len(v) == 0 {
			return msg
		}
		if !strings.Contains(msg, "%") {
			// do not contain format char
			msg += strings.Repeat(" %v", len(v))
		}
	default:
		msg = fmt.Sprint(f)
		if len(v) == 0 {
			return msg
		}
		msg += strings.Repeat(" %v", len(v))
	}
	return fmt.Sprintf(msg, v...)
}