package main

import (
	"context"
	"errors"
	"net"
	"time"

	"log/slog"

	"github.com/Microsoft/go-winio"
)

func handlePipeClients(ctx context.Context, logger *slog.Logger, ps net.Listener, broker *dataBroker) {
	logger = logger.With(slog.String(logKeyCategory, "pipelistener"))

	for {
		conn, err := ps.Accept()
		if err != nil {
			if !errors.Is(err, winio.ErrPipeListenerClosed) {
				logger.Error("failed to accept", slog.Any("err", err))
			}
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			logger.Info("Client connected to event pipe")

			eventStream := make(chan []byte, 10)
			broker.Add(eventStream)
			defer broker.Remove(eventStream)

			if err := handlePipeClient(ctx, c, eventStream); err != nil {
				logger.Error("failed to handle client", slog.Any("err", err))
			}
		}(conn)
	}
}

func handlePipeClient(ctx context.Context, conn net.Conn, events <-chan []byte) error {
	frequency := time.Second * 5
	ticker := time.NewTicker(frequency)
	pingMsg := []byte(`{"type":"ping"}`)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-events:
			if !ok {
				return nil
			}
			if err := writeLenPrefixed(conn, e); err != nil {
				return err
			}
			ticker.Reset(frequency)
		case <-ticker.C:
			if err := writeLenPrefixed(conn, pingMsg); err != nil {
				return err
			}
		}
	}
}
