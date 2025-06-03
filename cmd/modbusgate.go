package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type ModbusEntry struct {
	url       string
	connected bool
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("ModbusGate")

	var data = []*ModbusEntry{
		{url: "tcp://localhost:502", connected: false},
		{url: "tcp://localhost:503", connected: false},
	}
	list := widget.NewList(
		func() int {
			return len(data)
		},
		func() fyne.CanvasObject {
			// Create a template with a url and a button side by side
			url := widget.NewLabel("template")
			button := widget.NewButton("Connect", func() {})
			button.Importance = widget.DangerImportance
			return container.NewBorder(nil, nil, url, button)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			cont := o.(*fyne.Container)
			label := cont.Objects[0].(*widget.Label)
			button := cont.Objects[1].(*widget.Button)

			entry := data[i]
			label.SetText(entry.url)

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
