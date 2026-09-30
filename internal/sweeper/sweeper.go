package sweeper

import (
	"context"
	"log"
	"time"

	"github.com/abhishek-z2/shrnk/internal/store"
)

type Sweeper struct {
	store *store.PostgresStore
}

func New(store *store.PostgresStore) *Sweeper {
	return &Sweeper{
		store: store,
	}
}

func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.store.SweepExpired(ctx); err != nil {
				log.Printf("failed to delete expired urls: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
