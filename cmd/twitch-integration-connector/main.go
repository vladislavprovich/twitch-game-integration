package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"log/slog"

	"github.com/Microsoft/go-winio"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	appName        = "twitch-integration-connector"
	configFileName = appName + ".json"
	logKeyCategory = "category"
)

type eventPublisher interface {
	Publish(evt []byte)
}

func main() {
	level := slog.LevelInfo
	logLeveler := &level

	logFile := &lumberjack.Logger{
		Filename: appName + ".log",
		MaxAge:   14,
	}
	defer logFile.Close()

	slogHandler := slog.NewTextHandler(io.MultiWriter(os.Stdout, logFile), &slog.HandlerOptions{
		Level: logLeveler,
	})
	logger := slog.New(slogHandler)
	slog.SetDefault(logger)

	defer os.Stdout.Sync()

	cnf, err := readAndUpdateConfig()
	if err != nil {
		logger.Error("Error reading config", slog.Any("err", err))
		return
	}

	if cnf.Debug {
		*logLeveler = slog.LevelDebug
	}

	appCtx, appCtxCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer appCtxCancel()

	eventCh := make(chan []byte, 10)
	const (
		SIDEveryone          = `D:(A;;GWGR;;;WD)`
		SIDAllUsers          = `D:(A;;GWGR;;;AU)`
		SIDAllUsersNoNetwork = `D:(A;;GWGR;;;AU)(D;;GA;;;NS)`
		SIDInteractiveUser   = `D:(A;;GWGR;;;IU)`
	)
	pipePath := `\\.\pipe\__TwitchIntegration_Kirides_Conn`
	ps, err := winio.ListenPipe(pipePath, &winio.PipeConfig{MessageMode: true, SecurityDescriptor: SIDInteractiveUser})
	if err != nil {
		logger.Error("Could not setup pipe listener", slog.Any("err", err))
		return
	}

	services := newServiceManager(logger)

	defer ps.Close()
	services.Add("pipelistener stopping routine", func(ctx context.Context) {
		<-ctx.Done()
		ps.Close()
	})
	broker := newBroker()
	services.Add("data broker", func(ctx context.Context) {
		broker.Run(ctx)
		close(eventCh)
	})
	services.Add("pipelistener", func(ctx context.Context) {
		handlePipeClients(ctx, logger, ps, broker)
	})
	services.Add("stream elements", func(ctx context.Context) {
		if err := handleStreamElements(ctx, cnf.StreamElements, logger, broker); err != nil {
			logger.Error("failed to handle stream elements", slog.Any("err", err))
		}
	})
	services.Add("twitch chat", func(ctx context.Context) {
		if err := handleChat(ctx, cnf.Twitch, logger, broker); err != nil {
			logger.Error("failed to handle twitch chat", slog.Any("err", err))
		}
	})
	services.Add("twitch pubsub", func(ctx context.Context) {
		handlePubSub(ctx, logger, cnf.Twitch, broker)
	})
	<-appCtx.Done()
	logger.Info("Shutting down")

	services.Stop()
}

type loggerFn func(format string, args ...interface{})

func (l loggerFn) Printf(format string, args ...interface{}) {
	l(format, args...)
}

func prettyJson(data any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(data)
	return buf.Bytes(), err
}

func readAndUpdateConfig() (config, error) {
	confFilePath, err := filepath.Abs(configFileName)
	cnf := defaultConfig()
	if err != nil {
		return cnf, err
	}
	/*
		execPath, err := os.Executable()
		if err != nil {
			return cnf, err
		}
		confFilePath = filepath.Join(filepath.Dir(execPath), configFileName)
	*/

	writeConfigIndented := func(cnf config) error {
		data, err := prettyJson(cnf)
		if err != nil {
			return err
		}
		os.WriteFile(confFilePath, data, 0640)
		return nil
	}

	configContent, err := os.ReadFile(confFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := writeConfigIndented(cnf); err != nil {
				return cnf, err
			}
		}
	} else {
		if err := json.Unmarshal(configContent, &cnf); err != nil {
			return cnf, err
		}
		if err := writeConfigIndented(cnf); err != nil {
			return cnf, err
		}
	}
	return cnf, nil
}
