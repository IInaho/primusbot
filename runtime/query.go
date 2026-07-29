package runtime

import (
	"sort"
	"strings"
)

func (r *Manager) PendingApprovals() []ApprovalView {
	return r.approvals.Pending()
}

func (r *Manager) PendingQuestions() []QuestionView {
	return r.questions.Pending()
}

func (r *Manager) CurrentRunView() (RunView, bool) {
	return r.runs.CurrentRunView()
}

func (r *Manager) RunView(runID RunID) (RunView, bool) {
	return r.runs.RunView(runID)
}

func (r *Manager) ListRunViews(limit int) []RunView {
	return r.runs.ListRunViews(limit)
}

func (r *Manager) ArtifactView(runID RunID) (ArtifactView, bool) {
	return r.runs.ArtifactView(runID)
}

func (r *Manager) Stats() BotStats {
	return r.backend.Stats()
}

func (r *Manager) ProviderModel() (string, string) {
	return r.backend.ProviderModel()
}

func (r *Manager) SessionMessages() []DisplayMessage {
	return r.backend.SessionMessages()
}

func (r *Manager) CommandNames() []string {
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, name := range r.backend.CommandNames() {
		display := commandDisplayName(name)
		if display == "" || seen[display] {
			continue
		}
		seen[display] = true
		names = append(names, display)
	}
	for name := range r.runtimeCommands {
		display := commandDisplayName(name)
		if seen[display] {
			continue
		}
		seen[display] = true
		names = append(names, display)
	}
	sort.Strings(names)
	return names
}

func commandDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "$") {
		return name
	}
	return "/" + name
}
