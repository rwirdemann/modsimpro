package main

import (
	"strconv"

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

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("ModbusGate")

	var data = []*ModbusEntry{
		{url: "tcp://localhost:502", address: 100, connected: false},
		{url: "tcp://localhost:503", address: 101, connected: false},
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
				if entry.connected {
					println("Connected to:", entry.url)
				} else {
					println("Disconnected from:", entry.url)
				}
			}
		})

	// Empty container for the right side (2/3 of the window)
	rightSide := container.NewVBox()

	// Main split container with list on left (1/3) and empty space on right (2/3)
	split := container.NewHSplit(list, rightSide)
	split.SetOffset(0.33) // List takes up 1/3 of the width

	myWindow.Resize(fyne.NewSize(900, 600))
	myWindow.SetContent(split)
	myWindow.ShowAndRun()
}
