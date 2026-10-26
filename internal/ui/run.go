package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

// Run 启动界面，阻塞直到退出。
func Run(dir string, opts Options) error {
	// 中日韩 locale 下 go-runewidth 会把 ▾ │ ↑ 这类“模糊宽度”字符算成 2 列，
	// 而 tmux / 大多数终端按 1 列绘制，统一按 1 列算避免错位。
	runewidth.DefaultCondition.EastAsianWidth = false
	detectBackground()

	p := tea.NewProgram(New(dir, opts), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
