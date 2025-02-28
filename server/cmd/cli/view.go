package main

import (
	"fmt"
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
	s := "RSS-ZERO CLI" + "\n\n"

	for i, choice := range m.mainMenu {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			s += fmt.Sprintf("%s %s", cursor, choice) + "\n"
		} else {
			s += fmt.Sprintf("%s %s", cursor, choice) + "\n"
		}
	}

	s += "\n" + "Press q to exit"
	return s
}

func (m model) renderSubMenu() string {
	s := "RSS Feed Generator" + "\n\n"

	for i, choice := range m.subMenu {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			s += fmt.Sprintf("%s %s", cursor, choice) + "\n"
		} else {
			s += fmt.Sprintf("%s %s", cursor, choice) + "\n"
		}
	}

	s += "\n" + "Press q to exit"
	return s
}

func (m model) renderInput() string {
	var prompt string
	switch m.feedType {
	case "Zhihu Subscribe":
		prompt = "Please enter Zhihu user Url Token:"
	case "RSSHub Subscribe":
		prompt = "Please enter username:"
	case "GitHub Release Subscribe":
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
