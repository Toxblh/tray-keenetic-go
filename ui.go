package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// showSettingsWindow opens a window to manage all configured routers.
func showSettingsWindow(a fyne.App, routers []RouterConfig, onSave func([]RouterConfig)) {
	w := a.NewWindow("Settings — Keenetic Tray")
	w.Resize(fyne.NewSize(480, 320))
	w.CenterOnScreen()

	list := widget.NewList(
		func() int { return len(routers) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(routers[id].Name)
		},
	)

	selectedIdx := -1
	list.OnSelected = func(id widget.ListItemID) { selectedIdx = id }
	list.OnUnselected = func(id widget.ListItemID) { selectedIdx = -1 }

	addBtn := widget.NewButton("Add", func() {
		showRouterFormDialog(a, w, nil, routers, func(cfg RouterConfig, password string) {
			routers = append(routers, cfg)
			setPassword(cfg.Name, password)
			list.Refresh()
			onSave(routers)
		})
	})

	editBtn := widget.NewButton("Edit", func() {
		if selectedIdx < 0 || selectedIdx >= len(routers) {
			return
		}
		existing := routers[selectedIdx]
		showRouterFormDialog(a, w, &existing, routers, func(cfg RouterConfig, password string) {
			oldName := routers[selectedIdx].Name
			if oldName != cfg.Name {
				deletePassword(oldName)
			}
			routers[selectedIdx] = cfg
			setPassword(cfg.Name, password)
			list.Refresh()
			onSave(routers)
		})
	})

	deleteBtn := widget.NewButton("Delete", func() {
		if selectedIdx < 0 || selectedIdx >= len(routers) {
			return
		}
		name := routers[selectedIdx].Name
		dialog.ShowConfirm("Delete Router",
			fmt.Sprintf("Delete router '%s'?", name),
			func(ok bool) {
				if !ok {
					return
				}
				deletePassword(name)
				routers = append(routers[:selectedIdx], routers[selectedIdx+1:]...)
				selectedIdx = -1
				list.UnselectAll()
				list.Refresh()
				onSave(routers)
			}, w)
	})

	closeBtn := widget.NewButton("Close", w.Close)

	buttons := container.NewVBox(addBtn, editBtn, deleteBtn, widget.NewSeparator(), closeBtn)
	content := container.NewBorder(nil, nil, nil, buttons, list)
	w.SetContent(content)
	w.Show()
}

// showAddRouterDialog opens the add router dialog directly.
func showAddRouterDialog(a fyne.App, routers []RouterConfig, onSave func([]RouterConfig)) {
	w := a.NewWindow("Add Router — Keenetic Tray")
	w.Resize(fyne.NewSize(400, 280))
	w.CenterOnScreen()
	w.Show()

	showRouterFormDialog(a, w, nil, routers, func(cfg RouterConfig, password string) {
		routers = append(routers, cfg)
		setPassword(cfg.Name, password)
		onSave(routers)
		w.Close()
	})
}

// showRouterFormDialog shows a modal form for adding or editing a router.
func showRouterFormDialog(
	a fyne.App,
	parent fyne.Window,
	existing *RouterConfig,
	allRouters []RouterConfig,
	onConfirm func(cfg RouterConfig, password string),
) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("My Router")

	addressEntry := widget.NewEntry()
	addressEntry.SetPlaceHolder("192.168.1.1")

	loginEntry := widget.NewEntry()
	loginEntry.SetPlaceHolder("admin")

	passwordEntry := widget.NewPasswordEntry()

	originalName := ""
	if existing != nil {
		originalName = existing.Name
		nameEntry.SetText(existing.Name)
		addressEntry.SetText(existing.Address)
		loginEntry.SetText(existing.Login)
		if pw := getPassword(existing.Name); pw != "" {
			passwordEntry.SetText(pw)
		}
	}

	errorLabel := widget.NewLabel("")
	errorLabel.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Address", addressEntry),
		widget.NewFormItem("Login", loginEntry),
		widget.NewFormItem("Password", passwordEntry),
	)

	statusLabel := widget.NewLabel("")
	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	content := container.NewVBox(form, errorLabel, statusLabel, progress)

	var dlg dialog.Dialog
	dlg = dialog.NewCustomConfirm(
		"Router",
		"Save",
		"Cancel",
		content,
		func(save bool) {
			if !save {
				return
			}

			name := strings.TrimSpace(nameEntry.Text)
			address := strings.TrimSuffix(strings.TrimSpace(addressEntry.Text), "/")
			login := strings.TrimSpace(loginEntry.Text)
			password := passwordEntry.Text

			if name == "" || address == "" || login == "" || password == "" {
				errorLabel.SetText("Please fill in all fields.")
				// Re-show the dialog — Fyne doesn't re-show automatically, so we recreate
				showRouterFormDialog(a, parent, existing, allRouters, onConfirm)
				return
			}

			// Check for duplicate name
			for _, r := range allRouters {
				if r.Name == name && name != originalName {
					errorLabel.SetText("A router with this name already exists.")
					showRouterFormDialog(a, parent, existing, allRouters, onConfirm)
					return
				}
			}

			// Check connectivity in background
			progress.Show()
			statusLabel.SetText("Connecting...")
			dlg.Hide()

			checkWindow := a.NewWindow("Connecting...")
			checkWindow.Resize(fyne.NewSize(300, 100))
			checkWindow.CenterOnScreen()
			checkProg := widget.NewProgressBarInfinite()
			checkWindow.SetContent(container.NewVBox(
				widget.NewLabel("Verifying router connection..."),
				checkProg,
			))
			checkWindow.Show()

			go func() {
				router := NewKeeneticRouter(address, login, password, name)
				err := router.Login()

				cfg := RouterConfig{
					Name:    name,
					Address: address,
					Login:   login,
				}

				if err == nil {
					if ip, e := router.GetNetworkIP(); e == nil && ip != "" {
						cfg.NetworkIP = ip
					}
					if urls, e := router.GetKeenDNSURLs(); e == nil {
						cfg.KeenDNS = urls
					}
				}

				checkWindow.Close()

				if err != nil {
					showRouterFormDialogWithError(a, parent, existing, allRouters, onConfirm,
						nameEntry.Text, addressEntry.Text, loginEntry.Text, passwordEntry.Text,
						"Connection failed: "+err.Error())
					return
				}
				onConfirm(cfg, password)
			}()
		},
		parent,
	)

	dlg.Resize(fyne.NewSize(400, 300))
	dlg.Show()
}

// showRouterFormDialogWithError re-opens the form dialog pre-filled with an error message.
func showRouterFormDialogWithError(
	a fyne.App,
	parent fyne.Window,
	existing *RouterConfig,
	allRouters []RouterConfig,
	onConfirm func(cfg RouterConfig, password string),
	name, address, login, password, errMsg string,
) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(name)
	addressEntry := widget.NewEntry()
	addressEntry.SetText(address)
	loginEntry := widget.NewEntry()
	loginEntry.SetText(login)
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetText(password)

	errorLabel := widget.NewLabel(errMsg)
	errorLabel.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Address", addressEntry),
		widget.NewFormItem("Login", loginEntry),
		widget.NewFormItem("Password", passwordEntry),
	)

	content := container.NewVBox(form, errorLabel)

	originalName := ""
	if existing != nil {
		originalName = existing.Name
	}

	var dlg dialog.Dialog
	dlg = dialog.NewCustomConfirm("Router", "Save", "Cancel", content, func(save bool) {
		if !save {
			return
		}

		n := strings.TrimSpace(nameEntry.Text)
		addr := strings.TrimSuffix(strings.TrimSpace(addressEntry.Text), "/")
		lg := strings.TrimSpace(loginEntry.Text)
		pw := passwordEntry.Text

		if n == "" || addr == "" || lg == "" || pw == "" {
			showRouterFormDialogWithError(a, parent, existing, allRouters, onConfirm,
				nameEntry.Text, addressEntry.Text, loginEntry.Text, passwordEntry.Text,
				"Please fill in all fields.")
			return
		}
		for _, r := range allRouters {
			if r.Name == n && n != originalName {
				showRouterFormDialogWithError(a, parent, existing, allRouters, onConfirm,
					nameEntry.Text, addressEntry.Text, loginEntry.Text, passwordEntry.Text,
					"A router with this name already exists.")
				return
			}
		}

		dlg.Hide()
		checkWindow := a.NewWindow("Connecting...")
		checkWindow.Resize(fyne.NewSize(300, 100))
		checkWindow.CenterOnScreen()
		checkWindow.SetContent(container.NewVBox(
			widget.NewLabel("Verifying router connection..."),
			widget.NewProgressBarInfinite(),
		))
		checkWindow.Show()

		go func() {
			router := NewKeeneticRouter(addr, lg, pw, n)
			err := router.Login()
			cfg := RouterConfig{Name: n, Address: addr, Login: lg}
			if err == nil {
				if ip, e := router.GetNetworkIP(); e == nil && ip != "" {
					cfg.NetworkIP = ip
				}
				if urls, e := router.GetKeenDNSURLs(); e == nil {
					cfg.KeenDNS = urls
				}
			}
			checkWindow.Close()
			if err != nil {
				showRouterFormDialogWithError(a, parent, existing, allRouters, onConfirm,
					n, addr, lg, pw, "Connection failed: "+err.Error())
				return
			}
			onConfirm(cfg, pw)
		}()
	}, parent)

	dlg.Resize(fyne.NewSize(400, 300))
	dlg.Show()
}
