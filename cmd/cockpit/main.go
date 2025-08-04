// Simplemodbus provides a TUI to view and manipulate modbus register contents based on a register definition file.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rwirdemann/modsimpro"
	"github.com/rwirdemann/modsimpro/modbus"
)

const (
	focusRegisterList = iota
	focusRegisterInput
	focusSlaves
	panelHeight         = 10
	ratioLeftPanelWidth = 0.6
)

var (
	configPath *string // base directory of config files
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder())

var activeStyle = baseStyle.
	BorderForeground(lipgloss.Color("white"))

var passiveStyle = baseStyle.
	BorderForeground(lipgloss.Color("240"))

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#909090",
	Dark:  "#626262",
}).Padding(0, 1)

type slave struct {
	modsimpro.Slave
	url        string
	modbusPort modbusPort
	Registers  []modsimpro.Register
}

var slaves []slave

func main() {
	configPath = flag.String("config", "config", "config base directory")
	help := flag.Bool("help", false, "print usage")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	bb, err := os.ReadFile(path.Join(*configPath, "config.json"))
	if err != nil {
		log.Fatal(err)
	}
	if err := json.NewDecoder(bytes.NewReader(bb)).Decode(&config); err != nil {
		log.Fatal(err)
	}

	// Parse config into slave slices which is used by the view as its data model.
	for _, serial := range config.Serial {
		modbusPort := modbus.NewAdapter(serial)
		for _, s := range serial.Slaves {
			r := readFile(path.Join(*configPath, s.Type, "register.dsl"))
			register, err := parseRegisterDSL(r, s.Address)
			if err != nil {
				panic(err)
			}
			slaves = append(slaves, slave{
				Slave:      s,
				url:        serial.Url,
				modbusPort: modbusPort,
				Registers:  register,
			})
		}
	}

	defer func() {
		for _, s := range slaves {
			s.modbusPort.Close()
		}
	}()

	m := newModel()

	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

type modbusPort interface {
	ReadRegister(register []modsimpro.Register) []modsimpro.Register
	WriteRegister(register modsimpro.Register) error
	Close()
}

type model struct {
	focus            int
	registerTable    table.Model
	slaveTable       table.Model
	register         []modsimpro.Register
	currentRegister  modsimpro.Register
	registerInput    textinput.Model
	fullHeight       int
	fullWidth        int
	leftPanelWidth   int
	rightPanelWidth  int
	slavePanelHeight int
	editPanelHeight  int
}

func newModel() model {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)

	columns := []table.Column{
		{Title: "Slave", Width: 6},
		{Title: "Address", Width: 8},
		{Title: "Action", Width: 6},
		{Title: "Datatype", Width: 10},
		{Title: "Type", Width: 10},
		{Title: "Value", Width: 10},
	}

	registers := slaves[0].modbusPort.ReadRegister(slaves[0].Registers)
	rows := registersToTableRows(registers)
	registerTable := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)
	registerTable.SetStyles(s)

	propertyColumns := []table.Column{
		{Title: "URL", Width: 20},
		{Title: "Address", Width: 5},
		{Title: "Updated", Width: 9},
	}
	slaveTable := table.New(
		table.WithColumns(propertyColumns),
		// table.WithHeight(panelHeight-1),
		table.WithRows(slavesToTableRows()),
		table.WithFocused(true),
	)
	slaveTable.SetStyles(s)

	return model{registerTable: registerTable, registerInput: textinput.New(), focus: focusRegisterList, slaveTable: slaveTable, register: registers}
}

func slavesToTableRows() []table.Row {
	var rows []table.Row
	for _, s := range slaves {
		r := table.Row{s.url, fmt.Sprintf("%d", s.Address), time.Now().Format("15:04:05")}
		rows = append(rows, r)
	}
	return rows
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd { return tickCmd() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.fullHeight = msg.Height
		m.fullWidth = msg.Width

		m.leftPanelWidth = int(float32(m.fullWidth) * ratioLeftPanelWidth)
		m.rightPanelWidth = m.fullWidth - m.leftPanelWidth - 4

		m.slavePanelHeight = (m.fullHeight - 5) / 2
		m.editPanelHeight = (m.fullHeight - 5) / 2
		if m.fullHeight%2 != 0 {
			m.editPanelHeight -= 1
		}

		m.registerTable.SetHeight(m.fullHeight - 4)
		m.slaveTable.SetHeight(m.slavePanelHeight - 4)

		return m, nil

	case tea.KeyMsg:
		switch m.focus {
		case focusRegisterList:
			m.registerTable, cmd = m.registerTable.Update(msg)
			cmds = append(cmds, cmd)

			switch msg.String() {
			case "tab":
				m.focus = focusSlaves
				m.registerTable.Blur()
				m.slaveTable.Focus()
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter":
				m.currentRegister = m.register[m.registerTable.Cursor()]
				m.registerInput.SetValue(fmt.Sprintf("%v", m.currentRegister.RawData))
				m.registerInput.SetCursor(len(m.registerInput.Value()))
				m.registerInput.Focus()
				m.registerTable.Blur()
				m.focus = focusRegisterInput
			}

		case focusSlaves:
			oldCursor := m.slaveTable.Cursor()
			m.slaveTable, cmd = m.slaveTable.Update(msg)

			// Reset register cursor if slave has changed
			if oldCursor != m.slaveTable.Cursor() {
				m.registerTable.SetCursor(0)
			}
			cmds = append(cmds, cmd)

			switch msg.String() {
			case "tab":
				m.focus = focusRegisterList
				m.slaveTable.Blur()
				m.registerTable.Focus()
			}

		case focusRegisterInput:
			m.registerInput, cmd = m.registerInput.Update(msg)
			cmds = append(cmds, cmd)

			switch msg.String() {
			case "esc":
				m.registerTable.Focus()
				m.focus = focusRegisterList
			case "enter":
				switch m.currentRegister.RegisterType {
				case "discrete":
					m.currentRegister.Datatype = "BOOL"
					m.currentRegister.RawData = toBool(m.registerInput.Value())
				case "holding", "input":
					switch m.currentRegister.Datatype {
					case "T64T1234":
						m.currentRegister.RawData = toUnt64(m.registerInput.Value())
					case "F32T1234":
						m.currentRegister.RawData = toFloat32(m.registerInput.Value())
					case "F32T3412":
						m.currentRegister.RawData = toFloat32(m.registerInput.Value())
					}
				}
				err := slaves[m.slaveTable.Cursor()].modbusPort.WriteRegister(m.currentRegister)
				if err != nil {
					slog.Error(err.Error())
				}
				m.registerTable.Focus()
				m.focus = focusRegisterList
			}
		}
	case tickMsg:
		m.register = slaves[m.slaveTable.Cursor()].modbusPort.ReadRegister(slaves[m.slaveTable.Cursor()].Registers)
		cmds = append(cmds, tickCmd())
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	rows := registersToTableRows(slaves[m.slaveTable.Cursor()].modbusPort.ReadRegister(slaves[m.slaveTable.Cursor()].Registers))
	m.registerTable.SetRows(rows)
	m.slaveTable.SetRows(slavesToTableRows())
	configPanel := m.renderConfigTable()
	registerForm := m.renderRegisterForm()
	panels := lipgloss.JoinVertical(lipgloss.Top, configPanel, registerForm)
	registerTable := m.renderRegisterTable()
	return lipgloss.JoinHorizontal(lipgloss.Top, registerTable, panels)
}

func (m model) renderRegisterTable() string {
	var style lipgloss.Style
	if m.focus == focusRegisterList {
		style = activeStyle
	} else {
		style = passiveStyle
	}
	style = style.Height(m.fullHeight - 4).Width(m.leftPanelWidth)
	return style.Render(m.registerTable.View()) + "\n  " + m.registerTable.HelpView() + helpStyle.Render(" • <enter> update register value") + "\n"
}

func (m model) renderRegisterForm() string {
	var style lipgloss.Style
	if m.focus == focusRegisterInput {
		style = activeStyle
	} else {
		style = passiveStyle
	}

	s := ""
	if m.focus == focusRegisterInput {
		s = fmt.Sprintf("\nAddress: 0x%X\n", m.currentRegister.Address)
		s = fmt.Sprintf("%sType   : %s\n\n", s, m.currentRegister.RegisterType)
		m.registerInput.Prompt = "Value  : "
		s += m.registerInput.View()
	}

	style = style.Border(generateBorder("Edit Register", m.rightPanelWidth))
	return lipgloss.JoinVertical(
		lipgloss.Top,
		style.Padding(0, 1).Height(m.editPanelHeight).Width(m.rightPanelWidth).Render(s),
		helpStyle.Render("enter - save • esc - discard"))
}

func (m model) renderConfigTable() string {
	var style lipgloss.Style
	if m.focus == focusSlaves {
		style = activeStyle
	} else {
		style = passiveStyle
	}
	return style.Height(m.slavePanelHeight).Width(m.rightPanelWidth).Render(m.slaveTable.View())
}

func generateBorder(title string, width int) lipgloss.Border {
	if width < 0 {
		return lipgloss.RoundedBorder()
	}
	border := lipgloss.RoundedBorder()
	border.Top = border.Top + border.MiddleRight + " " + title + " " + border.MiddleLeft + strings.Repeat(border.Top, width)
	return border
}

func init() {
	slog.Info("Initializing main")
}

var config modsimpro.Config

func toFloat32(s string) float32 {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		slog.Error(err.Error())
		return 0
	}
	return float32(f)
}

func toBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		slog.Error(err.Error())
		return false
	}
	return b
}

func toUnt64(s string) uint64 {
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		slog.Error(err.Error())
		return 0
	}
	return i
}

func registersToTableRows(registers []modsimpro.Register) []table.Row {
	var rows []table.Row
	for _, r := range registers {
		rows = append(rows, buildTableRow(r))
	}
	return rows
}

func buildTableRow(r modsimpro.Register) table.Row {
	return table.Row{
		fmt.Sprintf("%d", r.SlaveAddress),
		fmt.Sprintf("0x%X", r.Address),
		r.Action,
		r.Datatype,
		r.RegisterType,
		fmt.Sprintf("%v", r.RawData),
	}
}

func readFile(name string) io.Reader {
	bb, err := os.ReadFile(name)
	if err != nil {
		log.Fatal(err)
	}
	return bytes.NewReader(bb)
}

func parseRegisterDSL(reader io.Reader, slaveAddress uint8) ([]modsimpro.Register, error) {
	dsl := readDSL(reader)
	var registers []modsimpro.Register

	for _, l := range dsl {
		line := strings.Trim(l, " ")

		// ignore empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !strings.HasPrefix(line, "read") && !strings.HasPrefix(line, "write") {
			return nil, fmt.Errorf("register.dsl: statement '%s' doesn't start with 'read' or 'write'", line)
		}
		ff := strings.Fields(line)
		if len(ff) != 6 {
			return nil, fmt.Errorf("register.dsl: statement '%s' contains invalid keywords", line)
		}
		reg := modsimpro.Register{
			SlaveAddress: slaveAddress,
			Action:       ff[0],
			Address:      parseUint16(ff[2]),
			Datatype:     ff[4],
			RegisterType: ff[5],
		}
		registers = append(registers, reg)
	}

	return registers, nil
}

func readDSL(r io.Reader) []string {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func parseUint16(s string) uint16 {
	i, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0
	}
	return uint16(i)
}
