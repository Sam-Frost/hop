package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	viewList viewState = iota
	viewForm
	viewSettings
	viewConfirmDelete
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(14)
	focusedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

const formFieldCount = 4 // Name, IP, User, Pem override

type model struct {
	cfg      Config
	state    viewState
	cursor   int // selected server index in list view
	err      string
	quitting bool

	// form state (add/edit server)
	editing    bool // true if editing an existing server, false if adding
	editIndex  int
	fields     [formFieldCount]string
	fieldFocus int

	// settings state
	settingsField string

	width, height int
}

func initialModel(cfg Config) model {
	return model{cfg: cfg, state: viewList}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case connectFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Sprintf("ssh exited with error: %v", msg.err)
		} else {
			m.err = ""
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case viewList:
			return m.updateList(msg)
		case viewForm:
			return m.updateForm(msg)
		case viewSettings:
			return m.updateSettings(msg)
		case viewConfirmDelete:
			return m.updateConfirmDelete(msg)
		}
	}
	return m, nil
}

// ---------- List view ----------

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.cfg.Servers)-1 {
			m.cursor++
		}

	case "a":
		m.state = viewForm
		m.editing = false
		m.editIndex = -1
		m.fields = [formFieldCount]string{}
		m.fieldFocus = 0
		m.err = ""

	case "e":
		if len(m.cfg.Servers) == 0 {
			return m, nil
		}
		s := m.cfg.Servers[m.cursor]
		m.state = viewForm
		m.editing = true
		m.editIndex = m.cursor
		m.fields = [formFieldCount]string{s.Name, s.IP, s.User, s.Pem}
		m.fieldFocus = 0
		m.err = ""

	case "d":
		if len(m.cfg.Servers) == 0 {
			return m, nil
		}
		m.state = viewConfirmDelete

	case "s":
		m.state = viewSettings
		m.settingsField = m.cfg.DefaultPem
		m.err = ""

	case "enter":
		if len(m.cfg.Servers) == 0 {
			return m, nil
		}
		s := m.cfg.Servers[m.cursor]
		pem := m.cfg.PemFor(s)
		return m, connectCmd(s, pem)
	}
	return m, nil
}

// ---------- Add/Edit form ----------

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = viewList
		m.err = ""
		return m, nil

	case "tab", "down":
		m.fieldFocus = (m.fieldFocus + 1) % formFieldCount
		return m, nil

	case "shift+tab", "up":
		m.fieldFocus = (m.fieldFocus - 1 + formFieldCount) % formFieldCount
		return m, nil

	case "enter":
		name := strings.TrimSpace(m.fields[0])
		ip := strings.TrimSpace(m.fields[1])
		if name == "" || ip == "" {
			m.err = "name and ip are required"
			return m, nil
		}
		s := Server{
			Name: name,
			IP:   ip,
			User: strings.TrimSpace(m.fields[2]),
			Pem:  strings.TrimSpace(m.fields[3]),
		}
		if m.editing {
			m.cfg.Servers[m.editIndex] = s
		} else {
			m.cfg.Servers = append(m.cfg.Servers, s)
		}
		if err := SaveConfig(m.cfg); err != nil {
			m.err = fmt.Sprintf("save failed: %v", err)
			return m, nil
		}
		m.state = viewList
		m.err = ""
		return m, nil

	case "backspace":
		f := m.fields[m.fieldFocus]
		if len(f) > 0 {
			m.fields[m.fieldFocus] = f[:len(f)-1]
		}
		return m, nil

	default:
		if len(msg.Runes) > 0 {
			m.fields[m.fieldFocus] += string(msg.Runes)
		}
		return m, nil
	}
}

// ---------- Settings (default pem) ----------

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = viewList
		return m, nil
	case "enter":
		m.cfg.DefaultPem = strings.TrimSpace(m.settingsField)
		if err := SaveConfig(m.cfg); err != nil {
			m.err = fmt.Sprintf("save failed: %v", err)
			return m, nil
		}
		m.state = viewList
		m.err = ""
		return m, nil
	case "backspace":
		if len(m.settingsField) > 0 {
			m.settingsField = m.settingsField[:len(m.settingsField)-1]
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			m.settingsField += string(msg.Runes)
		}
		return m, nil
	}
}

// ---------- Confirm delete ----------

func (m model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.cfg.Servers = append(m.cfg.Servers[:m.cursor], m.cfg.Servers[m.cursor+1:]...)
		if m.cursor >= len(m.cfg.Servers) && m.cursor > 0 {
			m.cursor--
		}
		if err := SaveConfig(m.cfg); err != nil {
			m.err = fmt.Sprintf("save failed: %v", err)
		}
		m.state = viewList
		return m, nil
	case "n", "esc":
		m.state = viewList
		return m, nil
	}
	return m, nil
}

// ---------- Connecting via ssh ----------

type connectFinishedMsg struct{ err error }

func connectCmd(s Server, pem string) tea.Cmd {
	args := []string{}
	if pem != "" {
		args = append(args, "-i", pem)
	}
	target := s.IP
	if s.User != "" {
		target = s.User + "@" + s.IP
	}
	args = append(args, target)

	c := exec.Command("ssh", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return connectFinishedMsg{err: err}
	})
}

// ---------- View ----------

func (m model) View() string {
	if m.quitting {
		return ""
	}
	switch m.state {
	case viewForm:
		return m.viewForm()
	case viewSettings:
		return m.viewSettings()
	case viewConfirmDelete:
		return m.viewConfirmDelete()
	default:
		return m.viewList()
	}
}

func (m model) viewList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH Manager") + "\n\n")

	if len(m.cfg.Servers) == 0 {
		b.WriteString("No servers yet. Press 'a' to add one.\n\n")
	} else {
		for i, s := range m.cfg.Servers {
			line := fmt.Sprintf("%-20s %-16s %s", s.Name, s.IP, pemLabel(m.cfg.PemFor(s)))
			if i == m.cursor {
				b.WriteString(selectedStyle.Render("> "+line) + "\n")
			} else {
				b.WriteString("  " + line + "\n")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("Default pem: %s\n\n", pemLabel(m.cfg.DefaultPem)))

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err) + "\n\n")
	}

	b.WriteString(helpStyle.Render("enter: connect  a: add  e: edit  d: delete  s: default pem  q: quit"))
	return b.String()
}

func pemLabel(p string) string {
	if p == "" {
		return "(none)"
	}
	return p
}

func (m model) viewForm() string {
	var b strings.Builder
	title := "Add Server"
	if m.editing {
		title = "Edit Server"
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	labels := []string{"Name", "IP", "User (opt.)", "Pem (opt.)"}
	for i, label := range labels {
		cursor := " "
		val := m.fields[i]
		if i == m.fieldFocus {
			cursor = ">"
			val += "█"
		}
		b.WriteString(fmt.Sprintf("%s %s %s\n", cursor, labelStyle.Render(label+":"), focusedStyle.Render(val)))
	}

	b.WriteString("\n")
	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err) + "\n\n")
	}
	b.WriteString(helpStyle.Render("tab/shift+tab: move  enter: save  esc: cancel"))
	return b.String()
}

func (m model) viewSettings() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Default Pem File") + "\n\n")
	b.WriteString(fmt.Sprintf("  %s%s█\n\n", labelStyle.Render("Pem path:"), focusedStyle.Render(m.settingsField)))
	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err) + "\n\n")
	}
	b.WriteString(helpStyle.Render("enter: save  esc: cancel"))
	return b.String()
}

func (m model) viewConfirmDelete() string {
	var b strings.Builder
	s := m.cfg.Servers[m.cursor]
	b.WriteString(titleStyle.Render("Delete Server") + "\n\n")
	b.WriteString(fmt.Sprintf("Delete %q (%s)? (y/n)\n", s.Name, s.IP))
	return b.String()
}

func mustExit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
