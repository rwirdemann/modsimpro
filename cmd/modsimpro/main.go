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
	"github.com/rwirdemann/modsimpro"
	"github.com/rwirdemann/modsimpro/modbus"
	"github.com/rwirdemann/panels"
)

var (
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
	Server *modsimpro.ModbusServer
}

func (c Slave) Description() string {
	return c.Name
}

func (c Slave) FilterValue() string {
	return c.URL + " " + c.Name
}

func (c Slave) Title() string {
	connected := " online"
	if !c.online {
		connected = "offline"
	}
	return fmt.Sprintf("%-20s %3d %-10s", c.URL, c.ID, connected)
}

type model struct {
	width, heigth int
	list          list.Model
	selected      int
	logger        *logger
	rootPanel     *panels.Panel
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Remove items from log panel if their number exceeds new panel height
		if len(m.logger.items) > 0 && len(m.logger.items) > msg.Height-3 {
			m.logger.items = m.logger.items[len(m.logger.items)-msg.Height-3:]
		}

		m.logger.maxItems = msg.Height - 3
		m.width = msg.Width
		m.heigth = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			if len(m.list.Items()) > 0 {
				selected := m.list.SelectedItem().(Slave)
				ts := time.Now().Format(time.DateTime)
				if selected.online {
					selected.Server.Disconnect(selected.ID)
					m.logger.Append(fmt.Sprintf("%s %s:%d: disconnected", ts, selected.URL, selected.ID))
				} else {
					selected.Server.Connect(selected.ID)
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
	help := helpStyle.Render("enter - connect • q - quit")
	return lipgloss.JoinVertical(lipgloss.Top, m.rootPanel.View(m, m.width, m.heigth), help)
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

func renderListView(m tea.Model, w, h int) string {
	model := m.(model)
	model.list.SetSize(w, h)
	return model.list.View()
}

func renderLogView(m tea.Model, _, _ int) string {
	model := m.(model)
	if len(model.logger.items) == 0 {
		return "logger.items is empty"
	}
	return strings.Join(model.logger.items, "\n")
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "/Users/ralfwirdemann/go/src/neonpulse.io/modbusappgo/config", "path to the configuration directory")
	flag.Parse()
	if configPath == "" {
		flag.PrintDefaults()
		os.Exit(0)
	}

	config, err := modbus.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	logger := &logger{}
	var connections []list.Item
	for _, serial := range config.Serial {
		ms := modsimpro.NewModbusServer(serial.Url, logger)
		err := ms.Start()
		if err != nil {
			log.Fatal(err)
		}

		for _, slave := range serial.Slaves {
			c := Slave{
				URL:    serial.Url,
				ID:     int(slave.Address),
				Name:   slave.Type,
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

	rootPanel := panels.NewPanel(panels.LayoutDirectionHorizontal, true, true, 1.0, nil)
	rootPanel.Append(panels.NewPanel(panels.LayoutDirectionNone, true, false, 0.35, renderListView))
	rootPanel.Append(panels.NewPanel(panels.LayoutDirectionNone, true, false, 0.65, renderLogView))
	m := model{
		list:      l,
		logger:    logger,
		rootPanel: rootPanel,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
