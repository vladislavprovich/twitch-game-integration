package main

import (
	"context"
	"sync"
	"time"

	"log/slog"
)

type serviceManager struct {
	running sync.Map
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
	logger    *slog.Logger
}

func newServiceManager(logger *slog.Logger) *serviceManager {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.With(logKeyCategory, "serviceManager")
	return &serviceManager{
		ctx:       ctx,
		ctxCancel: cancel,
		logger:    logger,
	}
}

func (s *serviceManager) Add(svc string, handler func(ctx context.Context)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.running.Store(svc, "")
		defer s.running.Delete(svc)
		handler(s.ctx)
	}()
}

func (s *serviceManager) Stop() error {
	s.ctxCancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	ticks := 0
	for {
		ticks++
		if ticks < 5 {
			ticker.Reset(time.Duration(10*ticks) * time.Millisecond)
		} else if ticks == 5 {
			ticker.Reset(time.Second)
		}
		<-ticker.C

		count := 0
		s.running.Range(func(key, _ interface{}) bool {
			s.logger.Info("Still running", "service", key)
			count++
			return true
		})
		if count == 0 {
			break
		}
	}

	return nil
}
