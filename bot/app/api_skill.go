package app

import (
	"fmt"

	commonview "nekocode/common/view"
)

// api_skill.go — Bot API：技能选择与技能/插件管理视图。

func (b *Bot) SelectSkill(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	sk, ok := b.ext.Skill(name)
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	b.cmd.SelectSkill(b.ctxMgr, sk.Context, name)
	b.ext.MarkSkillLoaded(name)
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
	return b.skillManagementView()
}

func (b *Bot) SetPluginEnabled(name string, enabled bool) (commonview.SkillManagementView, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.ext.SetPluginEnabled(name, enabled); err != nil {
		return commonview.SkillManagementView{}, err
	}
	return b.skillManagementView(), nil
}

func (b *Bot) RefreshSkillManagement() commonview.SkillManagementView {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ext.Reload()
	return b.skillManagementView()
}
