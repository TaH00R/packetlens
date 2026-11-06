package store

import (
	"context"
	"fmt"
	"time"
)

// Cleanup removes old cycles in bounded transactions. Cutoff is fixed for the
// entire sweep, so concurrent collection cannot keep a sweep running forever.
func Cleanup(ctx context.Context, s Store, cutoff time.Time) (int64, error) {
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := s.DeleteBefore(ctx, cutoff)
		total += n
		if err != nil {
			return total, fmt.Errorf("clean up expired history: %w", err)
		}
		if n < CleanupBatchSize {
			return total, nil
		}
	}
}
