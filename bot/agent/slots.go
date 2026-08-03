package agent

import (
	"fmt"
	"sync"
)

const maxSubSlots = 8

type slotManager struct {
	mu     sync.Mutex
	slots  [maxSubSlots]*subSlot
	active int
}

type subSlot struct {
	id      string
	subType string
}

func newSlotManager() *slotManager {
	return &slotManager{}
}

// Acquire reserves a sub-agent slot for TUI coloring/accounting. It never
// blocks: if all slots are taken it returns ok=false so the caller can fail
// the task fast instead of deadlocking the main goroutine (releases happen
// only after the whole tool batch finishes).
func (m *slotManager) Acquire(id, subType string) (colorIdx int, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active >= maxSubSlots {
		return -1, false
	}
	for i := range m.slots {
		if m.slots[i] == nil {
			m.slots[i] = &subSlot{id: id, subType: subType}
			m.active++
			return i, true
		}
	}
	return -1, false
}

func (m *slotManager) Release(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.slots {
		if m.slots[i] != nil && m.slots[i].id == id {
			m.slots[i] = nil
			m.active--
			return
		}
	}
}

func (m *slotManager) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for _, s := range m.slots {
		if s != nil {
			ids = append(ids, fmt.Sprintf("%s:%s", s.subType, s.id[:min(4, len(s.id))]))
		}
	}
	return fmt.Sprintf("SubSlotManager(%d/%d) %v", m.active, maxSubSlots, ids)
}
