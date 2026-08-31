package clock

import (
	"testing"
	"time"
)

func TestAdjustableSetAndReset(t *testing.T) {
	clock := NewAdjustable()
	target := time.Date(2030, 1, 2, 3, 4, 5, 0, time.FixedZone("UTC-5", -5*60*60))
	clock.Set(target)
	if !clock.Adjusted() {
		t.Fatal("clock should be marked as adjusted")
	}
	if difference := clock.Now().Sub(target); difference < 0 || difference > time.Second {
		t.Fatalf("adjusted time differs from target by %v", difference)
	}
	clock.Reset()
	if clock.Adjusted() {
		t.Fatal("clock should use real time after reset")
	}
	if difference := time.Since(clock.Now()); difference < -time.Second || difference > time.Second {
		t.Fatalf("reset clock differs from real time by %v", difference)
	}
}
