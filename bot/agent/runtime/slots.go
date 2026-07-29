package runtime

import (
	"fmt"
	"sync"

	"nekocode/bot/view"
)

const maxSubSlots = 8

type slotManager struct {
	mu     sync.Mutex
	cond   *sync.Cond
	slots  [maxSubSlots]*view.SubSlot
	active int
}

func newSlotManager() *slotManager {
	m := &slotManager{}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *slotManager) Acquire(id, subType string) (colorIdx int, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for m.active >= maxSubSlots {
		m.cond.Wait()
	}

	for i := range m.slots {
		if m.slots[i] == nil {
			m.slots[i] = &view.SubSlot{ID: id, SubType: subType, ColorIdx: i}
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
		if m.slots[i] != nil && m.slots[i].ID == id {
			m.slots[i] = nil
			m.active--
			m.cond.Signal()
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
			ids = append(ids, fmt.Sprintf("%s:%s", s.SubType, s.ID[:min(4, len(s.ID))]))
		}
	}
	return fmt.Sprintf("SubSlotManager(%d/%d) %v", m.active, maxSubSlots, ids)
}
