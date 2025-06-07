package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rwirdemann/modbusgate"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type ModbusEntry struct {
	url        string
	address    int
	connected  bool
	deviceType string
}

type logArea struct {
	textGrid           *widget.TextGrid
	logScrollContainer *container.Scroll
}

func newLogArea() *logArea {
	textGrid := widget.NewTextGrid()
	logScrollContainer := container.NewScroll(textGrid)
	logScrollContainer.SetMinSize(fyne.NewSize(400, 600))

	return &logArea{textGrid: textGrid, logScrollContainer: logScrollContainer}
}

func (l logArea) Append(text string) {
	l.textGrid.Append(text)
	l.logScrollContainer.ScrollToBottom()
}

var configPath string

func main() {
	flag.StringVar(&configPath, "config", "", "path to the configuration directory")
	flag.Parse()
	if configPath == "" {
		flag.PrintDefaults()
		os.Exit(0)
	}

	os.Exit(run())
}

func run() int {
	config, err := modbusgate.LoadConfig(configPath)
	if err != nil {
		slog.Error(err.Error())
		return 1
	}

	logArea := newLogArea()

	var ms *modbusgate.ModbusServer
	var data []*ModbusEntry
	for _, serial := range config.Serials {
		ms = modbusgate.NewModbusServer(serial.Url, logArea)
		err := ms.Start()
		if err != nil {
			slog.Error(err.Error())
			return 1
		}

		for _, slave := range serial.Slaves {
			data = append(data, &ModbusEntry{serial.Url, slave.Address, false, deviceTypeShort(slave.Type)})
		}
	}

	myApp := app.New()
	myWindow := myApp.NewWindow("ModbusGate")

	list := widget.NewList(
		func() int {
			return len(data)
		},
		func() fyne.CanvasObject {
			// Create a template with url, address and a button
			url := widget.NewLabel("template")
			address := widget.NewLabel("template")
			deviceType := widget.NewLabel("template")
			button := widget.NewButton("Connect", func() {})
			button.Importance = widget.DangerImportance

			left := container.NewHBox(url, address, deviceType)
			return container.NewBorder(nil, nil, left, button)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			cont := o.(*fyne.Container)
			leftContainer := cont.Objects[0].(*fyne.Container)
			button := cont.Objects[1].(*widget.Button)

			urlLabel := leftContainer.Objects[0].(*widget.Label)
			addressLabel := leftContainer.Objects[1].(*widget.Label)
			deviceType := leftContainer.Objects[2].(*widget.Label)

			entry := data[i]
			urlLabel.SetText(entry.url)
			addressLabel.SetText(strconv.Itoa(entry.address))
			deviceType.SetText(entry.deviceType)

			// Update button appearance based on connection state
			updateButton := func() {
				if entry.connected {
					button.SetText("Connected")
					button.Importance = widget.SuccessImportance // Green
				} else {
					button.SetText("Connect")
					button.Importance = widget.DangerImportance // Red
				}
				button.Refresh()
			}

			updateButton() // Initial state

			button.OnTapped = func() {
				// Toggle connection state
				entry.connected = !entry.connected
				updateButton()
				ts := time.Now().Format(time.DateTime)
				if entry.connected {
					logArea.Append(fmt.Sprintf("%s %s:%d: connected", ts, entry.url, entry.address))
					ms.Connect(entry.address)
				} else {
					logArea.Append(fmt.Sprintf("%s %s:%d: diconnected", ts, entry.url, entry.address))
					ms.Disconnect(entry.address)
				}
			}
		})



	rightSide := container.NewVBox()
	rightSide.Add(logArea.logScrollContainer)
	split := container.NewHSplit(list, rightSide)
	split.SetOffset(0.37)
	myWindow.Resize(fyne.NewSize(1200, 600))
	logArea.logScrollContainer.Resize(fyne.NewSize(logArea.logScrollContainer.Size().Width, 600))
	myWindow.SetContent(split)
	myWindow.ShowAndRun()
	return 0
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
