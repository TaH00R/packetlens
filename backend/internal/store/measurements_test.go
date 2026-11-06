package store

import (
	"context"
	"testing"
	"time"
)

func TestAnalyticsReaderDoesNotBlockCollection(t *testing.T) {
	s := openTestSQLite(t)
	if _, err := s.SaveSnapshot(context.Background(), sampleSnapshot(time.Now())); err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- s.WalkMeasurements(context.Background(), Filter{}, false, func(Measurement) error {
			select {
			case <-entered:
			default:
				close(entered)
			}
			<-release
			return nil
		})
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("analytics reader did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, err := s.SaveSnapshot(ctx, sampleSnapshot(time.Now()))
	cancel()
	close(release)
	if readErr := <-done; readErr != nil {
		t.Fatal(readErr)
	}
	if err != nil {
		t.Fatalf("analytics occupied the writer connection: %v", err)
	}
}
