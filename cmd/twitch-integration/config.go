package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

const (
	configFileName = "twitch-integration.json"
)

type chatCommand struct {
	Actions     []string `json:"actions"`
	CooldownSec int32    `json:"cooldown_sec"`
	Message     string   `json:"message"`
}

type config struct {
	Debug          bool           `json:"debug"`
	Twitch         twitch         `json:"twitch"`
	StreamElements streamElements `json:"streamElements"`
}
type twitch struct {
	Rewards map[string][]string    `json:"rewards"`
	Bits    map[int][]string       `json:"bits"`
	Chat    map[string]chatCommand `json:"chat"`
}
type streamElements struct {
	Perks map[string][]string `json:"perks"`
}

func defaultConfig() config {
	return config{
		Debug: false,
		StreamElements: streamElements{
			Perks: map[string][]string{
				"Item1": {"TWI_XXX", "TWI_YYY"},
			},
		},
		Twitch: twitch{
			Rewards: map[string][]string{
				"Item1": {"TWI_XXX", "TWI_YYY"},
			},
			Chat: map[string]chatCommand{
				"#help": {
					Actions:     []string{"XXXXXXXXXXXXXXXXXXXX"},
					Message:     "Dies hier wird im Chat angezeigt",
					CooldownSec: 5,
				},
			},
			Bits: map[int][]string{
				50:  {"TWI_XXX", "TWI_YYY"},
				100: {"TWI_XXX", "TWI_YYY"},
			},
		},
	}
}

func allKeysToUpper(m map[string][]string) map[string][]string {
	copy := make(map[string][]string, len(m))

	for k, v := range m {
		copy[strings.ToUpper(k)] = v
	}
	return copy
}

func prettyJson(data any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(data)
	return buf.Bytes(), err
}

func getConfigPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		app.logger.Error("failed to locate current executable", zap.Error(err))
		return "", err
	}

	return filepath.Join(filepath.Dir(execPath), configFileName), nil
}

func readConfigJson() (config, error) {
	cnf := defaultConfig()
	configPath, err := getConfigPath()
	if err != nil {
		app.logger.Error("failed to locate the config file", zap.Error(err))
		return cnf, err
	}

	configContent, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			def, err := prettyJson(cnf)
			if err != nil {
				app.logger.Error("failed to marshal default config", zap.Error(err))
				return cnf, err
			}
			os.WriteFile(configPath, def, 0640)
		}
	} else {
		if err := json.Unmarshal(configContent, &cnf); err != nil {
			app.logger.Error("failed to unmarshal config", zap.Error(err))
			return cnf, err
		}
	}

	if cnf.Twitch.Rewards == nil {
		cnf.Twitch.Rewards = make(map[string][]string)
	}
	if cnf.StreamElements.Perks == nil {
		cnf.StreamElements.Perks = make(map[string][]string)
	}
	cnf.Twitch.Rewards = allKeysToUpper(cnf.Twitch.Rewards)
	cnf.StreamElements.Perks = allKeysToUpper(cnf.StreamElements.Perks)

	if cnf.Twitch.Chat == nil {
		cnf.Twitch.Chat = make(map[string]chatCommand)
	}
	return cnf, nil
}

func watchForConfigChanges(ctx context.Context, watcher *fsnotify.Watcher, logger *zap.Logger) {
	execPath, err := os.Executable()
	if err != nil {
		logger.Error("failed to locate current executable", zap.Error(err))
		return
	}
	configDir := filepath.Dir(execPath)
	if err := watcher.Add(configDir); err != nil {
		logger.Error("failed to watch for config changes", zap.Error(err))
		return
	}

	// creates an encapsulated function that cancels any previous call to it
	fnTriggerReload, cancel := func() (func(), func()) {
		mtx := &sync.Mutex{}
		reloadCtx, cancel := context.WithCancel(ctx)

		return func() {
			mtx.Lock()
			cancel()
			reloadCtx, cancel = context.WithCancel(ctx)
			currentCtx := reloadCtx
			mtx.Unlock()

			select {
			case <-currentCtx.Done():
				return
			case <-time.After(time.Second):
				logger.Info("config file changed, reloading")
				if err := loadConfig(); err != nil {
					logger.Warn("reading the configuration yieled an error", zap.Error(err))
				}
			}
		}, func() { cancel() }
	}()

	go func() {
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) && strings.EqualFold(filepath.Base(event.Name), configFileName) {
					go fnTriggerReload()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Warn("fsnotify received an error", zap.Error(err))
			}
		}
	}()
}
