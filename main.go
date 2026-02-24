package main

import (
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
)

func main() {
	a := app.NewWithID("com.keenetic.tray")

	desk, ok := a.(desktop.App)
	if !ok {
		return
	}

	newTrayApp(a, desk)
	a.Run()
}
