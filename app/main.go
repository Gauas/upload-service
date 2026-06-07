package app

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"
)

type Server interface {
	Start(context.Context) error
}

func Start(servers ...Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	done := make(chan struct{})

	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(s Server) {
			defer wg.Done()
			if err := s.Start(ctx); err != nil {
				select {
				case errCh <- err:
					stop()
				default:
				}
			}
		}(server)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		<-done
		return fmt.Errorf("app stopped: %w", err)
	case <-ctx.Done():
		<-done
		return nil
	case <-done:
		return nil
	}
}
