package main

/*
#include <stdlib.h>
#include <debugapi.h>
*/
import "C"
import (
	"bytes"
	"container/list"
	"context"
	"os"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fsnotify/fsnotify"
)

var (
	funcQueue    = &strQueue{l: list.New(), mtx: new(sync.Mutex)}
	currentAlloc *C.char
	lastError    *C.char
	appCtx       context.Context
	appCtxCancel func()
	funcMtx      = new(sync.Mutex)
	activeCnf    *config
)

type strQueue struct {
	l   *list.List
	mtx *sync.Mutex
}

func (q *strQueue) Enq(v string) {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	q.l.PushBack(v)
}

func (q *strQueue) Deq() (string, bool) {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	if q.l.Len() == 0 {
		return "", false
	}
	e := q.l.Front()
	q.l.Remove(e)
	return e.Value.(string), true
}

func debugLog(str string) {
	debug := C.CString(str)
	defer C.free(unsafe.Pointer(debug))
	C.OutputDebugStringA(debug)
}

func enqueueEvent(fnc string) {
	app.logger.Debug("setCurrentFunc", zap.String("func", fnc))
	funcQueue.Enq(fnc)
}

func cSetStr(ppStorage **C.char, val string) *C.char {
	if *ppStorage != nil {
		C.free(unsafe.Pointer(*ppStorage))
	}
	*ppStorage = C.CString(val)
	return *ppStorage
}

func cError(err error) *C.char {
	return cSetStr(&lastError, err.Error())
}
func cErrorS(err string) *C.char {
	return cSetStr(&lastError, err)
}

type multiLogger []logger

func (m *multiLogger) Printf(format string, args ...interface{}) {
	for _, l := range *m {
		l.Printf(format, args...)
	}
}

type loggerFn func(format string, args ...interface{})

func (l loggerFn) Printf(format string, args ...interface{}) {
	l(format, args...)
}

type logger interface {
	Printf(format string, args ...interface{})
}

type outputDebugStringAWriter struct {
	back bytes.Buffer
}

func (w *outputDebugStringAWriter) Write(data []byte) (int, error) {
	n, err := w.back.Write(data)

	idx := bytes.IndexByte(w.back.Bytes(), '\n')
	for idx != -1 {
		debugLog(w.back.String()[:idx])

		rem := w.back.Bytes()[idx+1:]
		w.back.Reset()
		w.back.Write(rem)

		idx = bytes.IndexByte(w.back.Bytes(), '\n')
	}
	return n, err
}

type App struct {
	logger    *zap.Logger
	configMtx sync.Mutex
}

var app = &App{}

func (a *App) ReplaceConfig(c config) {

	a.configMtx.Lock()
	defer a.configMtx.Unlock()

	if activeCnf == nil {
		return
	}
	*activeCnf = c
}

func (a *App) GetConfig() config {
	a.configMtx.Lock()
	defer a.configMtx.Unlock()
	return *activeCnf
}

func loadConfig() error {
	cnf, err := readConfigJson()
	if err != nil {
		return err
	}

	app.ReplaceConfig(cnf)

	return nil
}

//export cInitIntegration
func cInitIntegration() *C.char {
	funcMtx.Lock()
	defer funcMtx.Unlock()
	if appCtxCancel != nil {
		return cErrorS("already running")
	}

	logFile, err := os.OpenFile("System/twitch-integration.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	out := zapcore.AddSync(&outputDebugStringAWriter{})
	if err == nil {
		out = zap.CombineWriteSyncers(logFile, out)
	}
	zapCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		out,
		zap.NewAtomicLevelAt(zap.DebugLevel))

	logger := zap.New(zapCore)
	defer logger.Sync()
	app.logger = logger

	enqueueEvent("INIT")

	cnf, err := readConfigJson()
	if err != nil {
		return cError(err)
	}
	activeCnf = &cnf

	appCtx, appCtxCancel = context.WithCancel(context.Background())

	go func() {
		if logFile != nil {
			defer logFile.Close()
		}

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			logger.Warn("failed to setup fsnotify to support config hot-reloading")
		} else {
			defer watcher.Close()
			watchForConfigChanges(appCtx, watcher, logger)
		}

		for {
			if err := handleEventPipe(appCtx, app, logger); err != nil {
				logger.Error("failed to run", zap.Error(err))
			}
			<-time.After(time.Second * 5)
		}
	}()
	enqueueEvent("START")
	return cErrorS("")
}

//export cShutdownIntegration
func cShutdownIntegration() *C.char {
	funcMtx.Lock()
	defer funcMtx.Unlock()
	if appCtxCancel == nil {
		return cErrorS("not running")
	}
	appCtxCancel()
	appCtxCancel = nil
	appCtx = nil
	activeCnf = nil
	return cErrorS("")
}

//export cHandleEvents
func cHandleEvents() *C.char {
	funcMtx.Lock()
	defer funcMtx.Unlock()
	if appCtx == nil {
		return cErrorS("not running")
	}
	cnf := activeCnf
	if cnf == nil {
		return cErrorS("no config")
	}

	fn, ok := funcQueue.Deq()
	if ok {
		app.logger.Info("func dequeued", zap.String("func", fn))
	}
	return cSetStr(&currentAlloc, fn)
}
