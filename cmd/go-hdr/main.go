package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Go HDR")

	// Control flag to ignore events triggered during startup/rendering
	isInitializing := true

	monitors := GetMonitors()
	content := container.NewVBox()

	if len(monitors) == 0 {
		content.Add(widget.NewLabel("No active monitors detected."))
	} else {
		// Fixed Column Widths
		colIDWidth := float32(100)
		colNameWidth := float32(140)
		colStatusWidth := float32(110)
		colToggleWidth := float32(80)

		createAlignedRow := func(wID, wName, wDevice, wStatus, wToggle fyne.CanvasObject) fyne.CanvasObject {
			leftSide := container.NewHBox(
				container.NewGridWrap(fyne.NewSize(colIDWidth, 36), wID),
				container.NewGridWrap(fyne.NewSize(colNameWidth, 36), wName),
			)
			rightSide := container.NewHBox(
				container.NewGridWrap(fyne.NewSize(colStatusWidth, 36), wStatus),
				container.NewGridWrap(fyne.NewSize(colToggleWidth, 36), wToggle),
			)

			deviceWithPadding := container.NewHBox(
				wDevice,
				container.NewGridWrap(fyne.NewSize(24, 36), widget.NewLabel("")),
			)

			return container.NewBorder(nil, nil, leftSide, rightSide, deviceWithPadding)
		}

		// Header Row
		headerID := widget.NewLabel("MONITOR ID")
		headerID.TextStyle = fyne.TextStyle{Bold: true}

		headerName := widget.NewLabel("NAME")
		headerName.TextStyle = fyne.TextStyle{Bold: true}

		headerDevice := widget.NewLabel("DEVICE")
		headerDevice.TextStyle = fyne.TextStyle{Bold: true}

		headerStatus := widget.NewLabel("STATUS")
		headerStatus.TextStyle = fyne.TextStyle{Bold: true}

		headerToggle := widget.NewLabel("TOGGLE")
		headerToggle.TextStyle = fyne.TextStyle{Bold: true}

		headerRow := createAlignedRow(headerID, headerName, headerDevice, headerStatus, headerToggle)
		content.Add(headerRow)
		content.Add(widget.NewSeparator())

		// Data Rows
		for _, mon := range monitors {
			m := mon // Local copy for closure safety

			displayNumLabel := widget.NewLabel(fmt.Sprintf("Display %d", m.ID+1))
			nameLabel := widget.NewLabel(m.MonitorName)
			deviceLabel := widget.NewLabel(m.DeviceName)

			statusStr := "SDR Mode"
			if m.IsHDROn {
				statusStr = "HDR Mode"
			}
			statusLabel := widget.NewLabel(statusStr)

			// Construct checkbox with callback guard
			check := widget.NewCheck("HDR", func(checked bool) {
				// Prevent Win32 API calls while constructing the UI
				if isInitializing {
					return
				}

				ToggleHDR(m.ID, checked)
				if checked {
					statusLabel.SetText("HDR Mode")
				} else {
					statusLabel.SetText("SDR Mode")
				}
			})

			// Set actual initial HDR state
			check.SetChecked(m.IsHDROn)

			row := createAlignedRow(displayNumLabel, nameLabel, deviceLabel, statusLabel, check)
			content.Add(row)
		}
	}

	paddedContent := container.NewPadded(content)
	myWindow.SetContent(paddedContent)
	myWindow.Resize(fyne.NewSize(780, 240))

	// Enable event execution right before entering the event loop
	isInitializing = false

	myWindow.ShowAndRun()
}
