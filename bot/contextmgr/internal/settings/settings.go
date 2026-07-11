package settings

import (
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/todo"
)

type Store struct {
	State *state.State
}

func (s *Store) SetSystemPrompt(systemPrompt string) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	s.State.Ctx.SystemPrompt = systemPrompt
}

func (s *Store) SetSkillList(skillList string) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	s.State.Ctx.Skills = skillList
}

func (s *Store) SetHints(hints string) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	s.State.Ctx.Hints = hints
}

func (s *Store) SetContextWindow(budget int) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	if budget > 0 {
		s.State.ContextWindow = budget
	}
}

func (s *Store) SetTodos(items []todo.Item) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	s.State.Ctx.LoadTodos(items)
}

func (s *Store) AllTasksDone() bool {
	s.State.Mu.RLock()
	defer s.State.Mu.RUnlock()
	return s.State.Ctx.AllTasksDone()
}

func (s *Store) HasTasks() bool {
	s.State.Mu.RLock()
	defer s.State.Mu.RUnlock()
	return s.State.Ctx.HasTasks()
}
