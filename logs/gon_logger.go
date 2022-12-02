package logs

import (
	"fmt"
	"sync"
	"errors"
	"os"
	"time"
	"runtime"
)

type nameLogger struct {
	Logger
	name string
}

// GonLogger is default logger in gon application.
// Can contain several providers and log message into all providers.
type GonLogger struct {
	lock                sync.Mutex
	level               int
	init                bool
	enableFuncCallDepth bool
	loggerFuncCallDepth int
	enableFullFilePath  bool
	asynchronous        bool
	prefix              string
	msgChanLen          int64
	msgChan             chan *LogMsg
	signalChan          chan string
	wg                  sync.WaitGroup
	outputs             []*nameLogger
	globalFormatter     string
}

const defaultAsyncMsgLen = 1e3
var logMsgPool *sync.Pool

// NewLogger returns new GonLogger
// channelLen: number of msg in chan(used when asynchronous is true)
// if buffering chan is full, logger adapter writes to file or other way
func NewLogger(channelLens ...int64) *GonLogger {
	gl := new(GonLogger)
	gl.level = LevelDebug
	gl.loggerFuncCallDepth = 3
	gl.msgChanLen = append(channelLens, 0)[0]
	if gl.msgChanLen <= 0 {
		gl.msgChanLen = defaultAsyncMsgLen
	}
	gl.signalChan = make(chan string, 1)
	gl.setLogger(AdapterConsole)
	return gl
}

// Async sets the log to asynchronous and start the goroutine
func (bl *GonLogger) Async(msgLen ...int64) *GonLogger {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	if bl.asynchronous {
		return bl
	}
	bl.asynchronous = true
	if len(msgLen) > 0 && msgLen[0] > 0 {
		bl.msgChanLen = msgLen[0]
	}
	bl.msgChan = make(chan *LogMsg, bl.msgChanLen)
	logMsgPool = &sync.Pool{
		New: func() interface{} {
			return &LogMsg{}
		},
	}
	bl.wg.Add(1)
	go bl.startLogger()
	return bl
}

func (bl *GonLogger) writeToLoggers(lm *LogMsg) {
	for _, l := range bl.outputs {
		err := l.WriteMsg(lm)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to WriteMsg to adapter:%v,error:%v\n", l.name, err)
		}
	}
}

// start logger chan reading.
// when chan is not empty, write logs.
func (bl *GonLogger) startLogger() {
	gameOver := false
	for {
		select {
		case bm := <-bl.msgChan:
			bl.writeToLoggers(bm)
			logMsgPool.Put(bm)
		case sg := <-bl.signalChan:
			// Now should only send "flush" or "close" to bl.signalChan
			bl.flush()
			if sg == "close" {
				for _, l := range bl.outputs {
					l.Destroy()
				}
				bl.outputs = nil
				gameOver = true
			}
			bl.wg.Done()
		}
		if gameOver {
			break
		}
	}
}

// SetLogger provides a logger adapter into GonLogger with config string.
// config must in in JSON format like {"interval":360}
func (gl *GonLogger) setLogger(adapterName string, configs ...string) error {
	config := append(configs, "{}")[0]
	for _, l := range gl.outputs {
		if l.name == adapterName {
			return fmt.Errorf("logs: duplicate adaptername %q (you have set this logger before)", adapterName)
		}
	}
	logAdapter, ok := adapters[adapterName]
	if !ok {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}

	lg := logAdapter()

	// Global formatter overrides the default set formatter
	if len(gl.globalFormatter) > 0 {
		fmtr, ok := GetFormatter(gl.globalFormatter)
		if !ok {
			return errors.New(fmt.Sprintf("the formatter with name: %s not found", gl.globalFormatter))
		}
		lg.SetFormatter(fmtr)
	}

	err := lg.Init(config)

	if err != nil {
		fmt.Fprintln(os.Stderr, "logs.GonLogger.SetLogger: "+err.Error())
		return err
	}
	gl.outputs = append(gl.outputs, &nameLogger{name: adapterName, Logger: lg})
	return nil
}

// SetLogger provides a given logger adapter into BeeLogger with config string.
// config must in in JSON format like {"interval":360}}
func (bl *GonLogger) SetLogger(adapterName string, configs ...string) error {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	if !bl.init {
		bl.outputs = []*nameLogger{}
		bl.init = true
	}
	return bl.setLogger(adapterName, configs...)
}

// DelLogger removes a logger adapter in GonLogger.
func (gl *GonLogger) DelLogger(adapterName string) error {
	gl.lock.Lock()
	defer gl.lock.Unlock()
	outputs := []*nameLogger{}
	for _, lg := range gl.outputs {
		if lg.name == adapterName {
			lg.Destroy()
		} else {
			outputs = append(outputs, lg)
		}
	}
	if len(outputs) == len(gl.outputs) {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}
	gl.outputs = outputs
	return nil
}

func (bl *GonLogger) writeMsg(lm *LogMsg) error {
	if !bl.init {
		bl.lock.Lock()
		bl.setLogger(AdapterConsole)
		bl.lock.Unlock()
	}

	var (
		file string
		line int
		ok   bool
	)

	_, file, line, ok = runtime.Caller(bl.loggerFuncCallDepth)
	if !ok {
		file = "???"
		line = 0
	}
	lm.FilePath = file
	lm.LineNumber = line

	lm.enableFullFilePath = bl.enableFullFilePath
	lm.enableFuncCallDepth = bl.enableFuncCallDepth

	// set level info in front of filename info
	if lm.Level == levelLoggerImpl {
		// set to emergency to ensure all log will be print out correctly
		lm.Level = LevelEmergency
	}

	if bl.asynchronous {
		logM := logMsgPool.Get().(*LogMsg)
		logM.Level = lm.Level
		logM.Msg = lm.Msg
		logM.When = lm.When
		logM.Args = lm.Args
		logM.FilePath = lm.FilePath
		logM.LineNumber = lm.LineNumber
		logM.Prefix = lm.Prefix
		if bl.outputs != nil {
			bl.msgChan <- lm
		} else {
			logMsgPool.Put(lm)
		}
	} else {
		bl.writeToLoggers(lm)
	}
	return nil
}

func (bl *GonLogger) flush() {
	if bl.asynchronous {
		for {
			if len(bl.msgChan) > 0 {
				bm := <-bl.msgChan
				bl.writeToLoggers(bm)
				logMsgPool.Put(bm)
				continue
			}
			break
		}
	}
	for _, l := range bl.outputs {
		l.Flush()
	}
}

func (gl *GonLogger) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// writeMsg will always add a '/n
	if p[len(p)-1] == '\n' {
		p = p[0 : len(p) - 1]
	}
	lm := &LogMsg{
		Msg: string(p),
		Level: levelLoggerImpl, // set levelLoggerImpl to ensure all log msg will be write out
	}

	err = gl.writeMsg(lm)
	if err == nil {
		return len(p), err
	}
	return 0, err
}

// GetLevel get current log message level
func (gl *GonLogger) GetLevel() int {
	return gl.level
}

func (gl *GonLogger) SetLogFuncCallDepth(d int) {
	gl.loggerFuncCallDepth = d
}

func (gl *GonLogger) GetLogFuncCallDepth() int {
	return gl.loggerFuncCallDepth
}

func (gl *GonLogger) EnableFuncCallDepth(b bool) {
	gl.enableFullFilePath = b
}

func (gl *GonLogger) SetPrefix(s string) {
	gl.prefix = s
}

func (gl *GonLogger) setGlobalFormatter(fmtter string) error {
	gl.globalFormatter = fmtter
	return nil
}

// Reset close all outputs and set gl.outputs to nil
func (gl *GonLogger) Reset() {
	gl.flush()
	for _, l := range gl.outputs {
		l.Destroy()
	}
	gl.outputs = nil
}

// Flush flush all chan data
func (gl *GonLogger) Flush() {
	if gl.asynchronous {
		gl.signalChan <- "flush"
		gl.wg.Wait()
		gl.wg.Add(1)
		return
	}
	gl.flush()
}

// Emergency Log EMERGENCY level message.
func (bl *GonLogger) Emergency(format string, v ...interface{}) {
	if LevelEmergency > bl.level {
		return
	}

	lm := &LogMsg{
		Level: LevelEmergency,
		Msg:   format,
		When:  time.Now(),
	}
	if len(v) > 0 {
		lm.Msg = fmt.Sprintf(lm.Msg, v...)
	}

	bl.writeMsg(lm)
}

// Alert Log ALERT level message.
func (bl *GonLogger) Alert(format string, v ...interface{}) {
	if LevelAlert > bl.level {
		return
	}

	lm := &LogMsg{
		Level: LevelAlert,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}
	bl.writeMsg(lm)
}

// Critical Log CRITICAL level message.
func (bl *GonLogger) Critical(format string, v ...interface{}) {
	if LevelCritical > bl.level {
		return
	}
	lm := &LogMsg{
		Level: LevelCritical,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}

// Error Log ERROR level message.
func (bl *GonLogger) Error(format string, v ...interface{}) {
	if LevelError > bl.level {
		return
	}
	lm := &LogMsg{
		Level: LevelError,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}

// Warning Log WARNING level message.
func (bl *GonLogger) Warning(format string, v ...interface{}) {
	lm := &LogMsg{
		Level: LevelWarning,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}

// Notice Log NOTICE level message.
func (bl *GonLogger) Notice(format string, v ...interface{}) {
	if LevelNotice > bl.level {
		return
	}
	lm := &LogMsg{
		Level: LevelNotice,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}

// Informational Log INFORMATIONAL level message.
func (bl *GonLogger) Informational(format string, v ...interface{}) {
	lm := &LogMsg{
		Level: LevelInformational,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}

// Debug Log DEBUG level message.
func (bl *GonLogger) Debug(format string, v ...interface{}) {
	lm := &LogMsg{
		Level: LevelDebug,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}

// Warn Log WARN level message.
// compatibility alias for Warning()
func (bl *GonLogger) Warn(format string, v ...interface{}) {
	lm := &LogMsg{
		Level: LevelWarning,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}

// Info Log INFO level message.
// compatibility alias for Informational()
func (bl *GonLogger) Info(format string, v ...interface{}) {
	lm := &LogMsg{
		Level: LevelInformational,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}

// Trace Log TRACE level message.
// compatibility alias for Debug()
func (bl *GonLogger) Trace(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	lm := &LogMsg{
		Level: LevelDebug,
		Msg:   format,
		When:  time.Now(),
		Args:  v,
	}

	bl.writeMsg(lm)
}