package runtime

import (
	"sync"
	"testing"
)

func TestTokenMeterZeroValue(t *testing.T) {
	var m tokenMeter
	prompt, completion := m.total(100)
	if prompt != 100 || completion != 0 {
		t.Fatalf("total() = (%d, %d), want (100, 0)", prompt, completion)
	}
	prompt, completion = m.turn(100)
	if prompt != 100 || completion != 0 {
		t.Fatalf("turn() before snapshot = (%d, %d), want (100, 0)", prompt, completion)
	}
}

func TestTokenMeterAccumulatesCompletion(t *testing.T) {
	var m tokenMeter
	m.add(10, 3)
	m.add(20, 4)

	// total's prompt comes from the caller's context occupancy, not from add.
	prompt, completion := m.total(500)
	if prompt != 500 {
		t.Fatalf("total() prompt = %d, want 500 (live context tokens)", prompt)
	}
	if completion != 7 {
		t.Fatalf("total() completion = %d, want 7", completion)
	}
}

func TestTokenMeterTurnReportsDeltaSinceSnapshot(t *testing.T) {
	var m tokenMeter
	m.add(0, 5)
	m.snapshot(1000)
	m.add(0, 8)

	prompt, completion := m.turn(1200)
	if prompt != 200 {
		t.Fatalf("turn() prompt = %d, want 200 (1200-1000)", prompt)
	}
	if completion != 8 {
		t.Fatalf("turn() completion = %d, want 8 (only post-snapshot tokens)", completion)
	}

	// A new snapshot resets the baseline; earlier deltas are excluded.
	m.snapshot(1200)
	m.add(0, 2)
	prompt, completion = m.turn(1500)
	if prompt != 300 || completion != 2 {
		t.Fatalf("turn() after re-snapshot = (%d, %d), want (300, 2)", prompt, completion)
	}
}

func TestTokenMeterConcurrentAdds(t *testing.T) {
	var m tokenMeter
	const workers = 8
	const perWorker = 1000

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				m.add(1, 2)
			}
		}()
	}
	wg.Wait()

	_, completion := m.total(0)
	if want := workers * perWorker * 2; completion != want {
		t.Fatalf("completion = %d, want %d", completion, want)
	}
}
