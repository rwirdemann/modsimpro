package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/rwirdemann/modbusgate"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type ModbusEntry struct {
	url       string
	address   int
	connected bool
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

	logArea := widget.NewTextGrid()

	var data []*ModbusEntry
	for _, serial := range config.Serials {
		ms := modbusgate.NewModbusServer(serial.Url, logArea)
		err := ms.Start()
		if err != nil {
			slog.Error(err.Error())
			return 1
		}

		for _, slave := range serial.Slaves {
			data = append(data, &ModbusEntry{serial.Url, slave.Address, false})
		}
	}

	myApp := app.New()
	myWindow := myApp.NewWindow("ModbusGate")

	logScrollContainer := container.NewScroll(logArea)
	logScrollContainer.SetMinSize(fyne.NewSize(400, 600))

	// Helper function to append text and auto-scroll to bottom
	appendAndScroll := func(text string) {
		logArea.Append(text)
		logScrollContainer.ScrollToBottom()
	}

	list := widget.NewList(
		func() int {
			return len(data)
		},
		func() fyne.CanvasObject {
			// Create a template with url, address and a button
			url := widget.NewLabel("template")
			address := widget.NewLabel("template")
			button := widget.NewButton("Connect", func() {})
			button.Importance = widget.DangerImportance
			return container.NewHBox(url, address, button)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			cont := o.(*fyne.Container)
			urlLabel := cont.Objects[0].(*widget.Label)
			addressLabel := cont.Objects[1].(*widget.Label)
			button := cont.Objects[2].(*widget.Button)

			entry := data[i]
			urlLabel.SetText(entry.url)
			addressLabel.SetText(strconv.Itoa(entry.address))

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
					appendAndScroll(fmt.Sprintf("%s %s:%d: connected", ts, entry.url, entry.address))
				} else {
					appendAndScroll(fmt.Sprintf("%s %s:%d: diconnected", ts, entry.url, entry.address))
				}
			}
		})

	// Empty container for the right side (2/3 of the window)
	rightSide := container.NewVBox()
	rightSide.Add(logScrollContainer)

	// Main split container with list on left (1/3) and empty space on right (2/3)
	split := container.NewHSplit(list, rightSide)
	split.SetOffset(0.33) // List takes up 1/3 of the width

	myWindow.Resize(fyne.NewSize(900, 600))
	myWindow.SetContent(split)
	myWindow.ShowAndRun()
	return 0
}
