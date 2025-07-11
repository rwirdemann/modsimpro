package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rwirdemann/modbusgate"
)

var (
	itemStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				PaddingRight(1).
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#F25D94"))

	slavePanelStyle = lipgloss.NewStyle().Height(20).Width(49).Border(lipgloss.NormalBorder())
	logPanelStyle   = lipgloss.NewStyle().Height(20).Width(70).Border(lipgloss.NormalBorder())

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#626262",
	}).Padding(0, 1)
)

// Slave represents an entry in the slave list. A slave holds a reference to the server it belongs to in order to inform
// the server wether the slave is online or not.
type Slave struct {
	URL    string
	ID     int
	Name   string
	online bool
	Server *modbusgate.ModbusServer
}

func (c Slave) Description() string {
	connected := " online"
	if !c.online {
		connected = "offline"
	}
	return fmt.Sprintf("%-20s %3d %15s %-10s", c.URL, c.ID, c.Name, connected)
}

func (c Slave) FilterValue() string {
	return c.URL + " " + c.Name
}

type model struct {
	list     list.Model
	selected int
	quitting bool
	logger   *logger
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func panelHeight(height int) int {
	return height - 3
}

func (m model) panelWidth(windowWidth int) (int, int) {
	slavePanelWidth := float32(windowWidth) * 0.40
	logPanelWidth := float32(windowWidth) * 0.60
	return int(slavePanelWidth), int(logPanelWidth) - 3
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		panelHeight := panelHeight(msg.Height)

		// Remove items from log panel if their number exceeds new panel height
		if len(m.logger.items) > panelHeight {
			m.logger.items = m.logger.items[len(m.logger.items)-panelHeight:]
		}

		m.logger.maxItems = panelHeight
		slavePanelWidth, logPanelWidth := m.panelWidth(msg.Width)
		slavePanelStyle = slavePanelStyle.Width(slavePanelWidth)
		slavePanelStyle = slavePanelStyle.Height(panelHeight)
		logPanelStyle = logPanelStyle.Width(logPanelWidth)
		logPanelStyle = logPanelStyle.Height(panelHeight)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if len(m.list.Items()) > 0 {
				selected := m.list.SelectedItem().(Slave)
				if selected.online {
					selected.Server.Disconnect(selected.ID)
					ts := time.Now().Format(time.DateTime)
					m.logger.Append(fmt.Sprintf("%s %s:%d: disconnected", ts, selected.URL, selected.ID))
				} else {
					selected.Server.Connect(selected.ID)
					ts := time.Now().Format(time.DateTime)
					m.logger.Append(fmt.Sprintf("%s %s:%d: connected", ts, selected.URL, selected.ID))
				}
				selected.online = !selected.online
				return m, m.list.SetItem(m.list.Index(), selected)
			}
			return m, nil
		}
	case tickMsg:
		cmds = append(cmds, tickCmd())
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var b strings.Builder

	// Connection list
	for i, item := range m.list.Items() {
		conn := item.(Slave)

		var style lipgloss.Style
		if i == m.list.Index() {
			style = selectedItemStyle
		} else {
			style = itemStyle
		}

		b.WriteString(style.Render(conn.Description()))
		b.WriteString("\n")
	}

	var logs = logPanelStyle.Render(strings.Join(m.logger.items, "\n"))
	s := lipgloss.JoinHorizontal(lipgloss.Top, slavePanelStyle.Render(b.String()), logs)
	help := helpStyle.Render("enter - connect • q - quit")
	return lipgloss.JoinVertical(lipgloss.Top, s, help)
}

type logger struct {
	items    []string
	maxItems int
}

func (l *logger) Append(s string) {
	l.items = append(l.items, s)
	if len(l.items) > l.maxItems {
		l.items = l.items[1:]
	}
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "/Users/ralfwirdemann/go/src/neonpulse.io/modbusappgo/config", "path to the configuration directory")
	flag.Parse()
	if configPath == "" {
		flag.PrintDefaults()
		os.Exit(0)
	}

	config, err := modbusgate.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	logger := &logger{}
	var connections []list.Item
	for _, serial := range config.Serials {
		ms := modbusgate.NewModbusServer(serial.Url, logger)
		err := ms.Start()
		if err != nil {
			log.Fatal(err)
		}

		for _, slave := range serial.Slaves {
			c := Slave{
				URL:    serial.Url,
				ID:     slave.Address,
				Name:   deviceTypeShort(slave.Type),
				Server: ms,
			}
			connections = append(connections, c)
		}
	}

	l := list.New(connections, list.NewDefaultDelegate(), 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)

	m := model{
		list:   l,
		logger: logger,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func deviceTypeShort(s string) string {
	if strings.Contains(s, "shortcircuit") {
		return "shortcircuit"
	}
	if strings.Contains(s, "trafo") {
		return "trafo"
	}
	return "unknown"
}
