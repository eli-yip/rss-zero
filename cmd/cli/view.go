package main

import (
	"fmt"
	"strings"
)

func (m model) View() string {
	switch m.state {
	case StateMainMenu:
		return m.renderMainMenu()
	case StateSubMenu:
		return m.renderSubMenu()
	case StateInput:
		return m.renderInput()
	case StateResult:
		return m.renderResult()
	default:
		return "Unknown state"
	}
}

func (m model) renderMainMenu() string {
	var s strings.Builder
	s.WriteString("RSS-ZERO CLI" + "\n\n")

	for i, choice := range m.mainMenu {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			s.WriteString(fmt.Sprintf("%s %s", cursor, choice) + "\n")
		} else {
			s.WriteString(fmt.Sprintf("%s %s", cursor, choice) + "\n")
		}
	}

	s.WriteString("\n" + "Press q to exit")
	return s.String()
}

func (m model) renderSubMenu() string {
	var s strings.Builder
	s.WriteString("RSS Feed Generator" + "\n\n")

	for i, choice := range m.subMenu {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			s.WriteString(fmt.Sprintf("%s %s", cursor, choice) + "\n")
		} else {
			s.WriteString(fmt.Sprintf("%s %s", cursor, choice) + "\n")
		}
	}

	s.WriteString("\n" + "Press q to exit")
	return s.String()
}

func (m model) renderInput() string {
	var prompt string
	switch m.feedType {
	case feedTypeZhihu:
		prompt = "Please enter Zhihu user Url Token:"
	case feedTypeRSSHub:
		prompt = "Please enter username:"
	case feedTypeGitHub:
		prompt = "Please enter repository address in user/repo format:"
	}

	return "RSS Feed Generator" + "\n\n" +
		prompt + "\n" +
		m.input.View() + "\n\n" +
		"Press Enter to confirm, press q to exit"
}

func (m model) renderResult() string {
	if m.err != nil {
		return "Error" + "\n\n" +
			m.err.Error() + "\n\n" +
			"Press Ctrl+C to exit"
	}

	return "Generation Successful" + "\n\n" +
		m.result + "\n\n" +
		"Press Ctrl+C to exit"
}
