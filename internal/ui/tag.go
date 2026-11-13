package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xBB2B/gitpane/internal/gitx"
	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 打标签：在图谱里选中一个提交，按 T 或右键选「打标签…」，输入标签名，
// 回车创建并推送（或点「仅在本地创建」）。之后点「↑ 推送」时也会顺带推送新建的标签。

// beginTag 打开标签输入框，并根据已有标签建议下一个版本号。
func (m *Model) beginTag() tea.Cmd {
	if m.repo == nil {
		return nil
	}
	i := m.cursor[tabGraph]
	if m.tab != tabGraph || i < 0 || i >= len(m.graph) || m.graph[i].Hash == "" {
		m.setMsg(lvInfo, i18n.T("在图谱里选中一个提交，再按 T 打标签"))
		return nil
	}
	c := m.graph[i]
	tags, _ := m.repo.Tags()
	m.tagRev, m.tagShort, m.tagSuggest = c.Full, c.Hash, gitx.SuggestTag(tags)
	m.tagging = true
	m.input.SetValue("")
	m.input.Placeholder = i18n.Tf("给 %s 打标签，比如 %s（Tab 填入）· 回车创建并推送 · Esc 取消", c.Hash, m.tagSuggest)
	m.input.Width = max(10, m.w-14)
	return m.input.Focus()
}

func (m *Model) endTag() {
	m.tagging = false
	m.input.Blur()
	m.input.Placeholder = i18n.T("写点什么…  回车提交 · Esc 取消") // 还原成提交信息的提示
}

func (m Model) handleTagKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "ctrl+c":
		m.endTag()
		return m, nil
	case "tab":
		if strings.TrimSpace(m.input.Value()) == "" {
			m.input.SetValue(m.tagSuggest)
			m.input.CursorEnd()
		}
		return m, nil
	case "enter":
		return m, m.doTag(true)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	return m, cmd
}

// doTag 创建标签；push 为 true 时接着推送到远程。
func (m *Model) doTag(push bool) tea.Cmd {
	name := strings.TrimSpace(m.input.Value())
	if name == "" {
		name = m.tagSuggest // 什么都没输入就回车：用建议的版本号
	}
	repo, rev := m.repo, m.tagRev
	if err := repo.CheckTagName(name); err != nil {
		m.setMsg(lvErr, err.Error())
		return nil
	}
	m.endTag()
	label := i18n.T("打标签中…")
	if push {
		label = i18n.T("打标签并推送中…")
	}
	return m.startOp(label, func() (string, error) {
		if err := repo.CreateTag(name, rev); err != nil {
			return "", err
		}
		if !push {
			return i18n.Tf("已创建标签 %s（推送分支时会一起推送）", name), nil
		}
		if err := repo.PushTag(name); err != nil {
			return "", fmt.Errorf("%s：%s", i18n.Tf("标签 %s 已在本地创建，但推送失败", name), gitx.Describe(err))
		}
		return i18n.Tf("已创建并推送标签 %s", name), nil
	})
}

func (m Model) tagButtons() []button {
	return place(1, m.w, []button{
		{id: "tagPush", label: "◆ " + i18n.T("创建并推送"), kind: btnPrimary},
		{id: "tagLocal", label: i18n.T("仅在本地创建")},
		{id: "tagCancel", label: i18n.T("取消")},
	})
}
