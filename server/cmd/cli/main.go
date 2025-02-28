package main

import (
	"fmt"
	"os"

	"github.com/eli-yip/rss-zero/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if err := config.InitForTestToml(); err != nil {
		fmt.Printf("Configuration initialization failed: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Program execution error: %v\n", err)
		os.Exit(1)
	}
}

const (
	StateMainMenu = iota
	StateSubMenu
	StateInput
	StateResult
)

const (
	MenuItemRSSFeed = iota
)

const (
	SubMenuZhihu = iota
	SubMenuRSSHub
	SubMenuGitHub
)

type model struct {
	state     int
	cursor    int
	mainMenu  []string
	subMenu   []string
	input     textinput.Model
	result    string
	feedType  string
	serverURL string
	err       error
	width     int
}

type feedResultMsg struct {
	result string
	err    error
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Please input..."
	ti.Focus()

	return model{
		state:     StateMainMenu,
		mainMenu:  []string{"RSS Feed Generator"},
		subMenu:   []string{"Zhihu Subscribe", "RSSHub Subscribe", "GitHub Release Subscribe"},
		input:     ti,
		serverURL: config.C.Settings.ServerURL,
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			switch m.state {
			case StateMainMenu:
				if m.cursor == MenuItemRSSFeed {
					m.state = StateSubMenu
					m.cursor = 0
					return m, nil
				}
				return m, tea.Quit
			case StateSubMenu:
				m.state = StateInput
				m.feedType = m.subMenu[m.cursor]
				return m, nil
			case StateInput:
				return m, m.generateFeed
			default:
				return m, tea.Quit
			}

		case "up", "k":
			if m.state == StateMainMenu && m.cursor > 0 {
				m.cursor--
			} else if m.state == StateSubMenu && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.state == StateMainMenu && m.cursor < len(m.mainMenu)-1 {
				m.cursor++
			} else if m.state == StateSubMenu && m.cursor < len(m.subMenu)-1 {
				m.cursor++
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case feedResultMsg:
		m.state = StateResult
		m.result = msg.result
		m.err = msg.err
		return m, nil
	}

	if m.state == StateInput {
		m.input, cmd = m.input.Update(msg)
	}

	return m, cmd
}
