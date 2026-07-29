package app

import (
	"fmt"

	commonview "nekocode/common/view"
)

// api_skill.go — Bot API：技能选择与技能/插件管理视图。

func (b *Bot) SelectSkill(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	skills := skillCommandProvider{manager: b.ext.skills}
	sk, ok := skills.GetForCommand(name)
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	b.cmd.SelectSkill(b.ctxMgr, sk.Context, name)
	skills.MarkLoaded(name)
	return nil
}

func (b *Bot) ClearSelectedSkill() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cmd.ClearSkill(b.ctxMgr)
}

func (b *Bot) SkillManagementView() commonview.SkillManagementView {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.SkillManagementView()
}

func (b *Bot) SetPluginEnabled(name string, enabled bool) (commonview.SkillManagementView, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.SetPluginEnabled(name, enabled)
}

func (b *Bot) RefreshSkillManagement() commonview.SkillManagementView {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.RefreshSkillManagement()
}
