package container

import (
	"context"
	"log"
	"time"

	"github.com/scriptertoufiq/go-mvc/internal/repositories"
)

// tokenSweepInterval is how often expired refresh tokens are purged. Hourly is
// ample: rows only become eligible once they pass JWT_REFRESH_TTL (30 days by
// default), so there is nothing to gain from sweeping more aggressively.
const tokenSweepInterval = time.Hour

// startTokenSweeper purges refresh tokens that can no longer be presented.
//
// Without this the table only ever grows: every login adds a row, and rotation
// adds one per refresh. Revoked and expired rows have no remaining value —
// IsUsable already rejects them — so they are hard-deleted rather than soft.
func startTokenSweeper(repo repositories.RefreshTokenRepository, stop <-chan struct{}) {
	sweep := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		removed, err := repo.DeleteExpired(ctx, time.Now())
		if err != nil {
			log.Printf("cleanup: purging expired refresh tokens failed: %v", err)
			return
		}
		if removed > 0 {
			log.Printf("cleanup: purged %d expired refresh token(s)", removed)
		}
	}

	go func() {
		ticker := time.NewTicker(tokenSweepInterval)
		defer ticker.Stop()

		// Sweep once at boot so a long-stopped instance doesn't wait an hour to
		// clear whatever accumulated while it was down.
		sweep()

		for {
			select {
			case <-ticker.C:
				sweep()
			case <-stop:
				return
			}
		}
	}()
}
