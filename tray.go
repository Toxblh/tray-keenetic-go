package main

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

const (
	refreshInterval     = 10 * time.Second
	maxInterfacesInline = 2
)

// appState holds the result of a background state collection.
type appState struct {
	router      *KeeneticRouter
	routerInfo  *RouterConfig
	interfaces  []InterfaceInfo
	policies    map[string]interface{}
	activeIface *InterfaceInfo
}

// TrayApp manages the system tray icon and menu.
type TrayApp struct {
	app  fyne.App
	desk desktop.App

	mu      sync.Mutex
	routers []RouterConfig

	state     *appState
	refreshCh chan struct{}
	stopCh    chan struct{}
	ticker    *time.Ticker
}

func newTrayApp(a fyne.App, desk desktop.App) *TrayApp {
	t := &TrayApp{
		app:       a,
		desk:      desk,
		routers:   loadRouters(),
		refreshCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}

	desk.SetSystemTrayIcon(GenerateIcon("---"))
	t.setLoadingMenu()

	t.ticker = time.NewTicker(refreshInterval)
	go t.loop()

	t.scheduleRefresh()
	return t
}

func (t *TrayApp) scheduleRefresh() {
	select {
	case t.refreshCh <- struct{}{}:
	default:
	}
}

func (t *TrayApp) loop() {
	for {
		select {
		case <-t.stopCh:
			return
		case <-t.ticker.C:
			t.scheduleRefresh()
		case <-t.refreshCh:
			state := t.collectState()
			t.applyState(state)
		}
	}
}

func (t *TrayApp) collectState() *appState {
	t.mu.Lock()
	routers := make([]RouterConfig, len(t.routers))
	copy(routers, t.routers)
	t.mu.Unlock()

	if len(routers) == 0 {
		return &appState{}
	}

	localNetworks := getLocalNetworks()
	defaultIface := getDefaultInterfaceName()

	for i := range routers {
		ri := &routers[i]
		password := getPassword(ri.Name)
		if password == "" {
			continue
		}

		// Check if the router is reachable on the current network.
		addr := ""
		if ri.NetworkIP != "" && isIPInNetworks(ri.NetworkIP, localNetworks) {
			addr = ri.NetworkIP
		} else {
			host := extractHost(ri.Address)
			if host != "" && isIPInNetworks(host, localNetworks) {
				addr = ri.Address
			}
		}
		if addr == "" {
			continue
		}

		router := NewKeeneticRouter(addr, ri.Login, password, ri.Name)
		if err := router.Login(); err != nil {
			continue
		}

		policies, _ := router.GetPolicies()
		if policies == nil {
			policies = map[string]interface{}{}
		}
		clients, _ := router.GetOnlineClients()

		interfaces := listLocalInterfaces(clients)
		if len(interfaces) > maxInterfacesInline {
			if defaultIface != "" {
				for _, iface := range interfaces {
					if iface.Name == defaultIface {
						interfaces = []InterfaceInfo{iface}
						break
					}
				}
			}
			if len(interfaces) > maxInterfacesInline {
				interfaces = interfaces[:1]
			}
		}

		var activeIface *InterfaceInfo
		for j := range interfaces {
			if defaultIface != "" && interfaces[j].Name == defaultIface {
				activeIface = &interfaces[j]
				break
			}
		}
		if activeIface == nil && len(interfaces) > 0 {
			activeIface = &interfaces[0]
		}

		return &appState{
			router:      router,
			routerInfo:  ri,
			interfaces:  interfaces,
			policies:    policies,
			activeIface: activeIface,
		}
	}
	return &appState{}
}

func (t *TrayApp) applyState(state *appState) {
	t.state = state

	t.mu.Lock()
	hasRouters := len(t.routers) > 0
	t.mu.Unlock()

	if !hasRouters {
		t.setNoRoutersMenu()
		t.desk.SetSystemTrayIcon(GenerateIcon("---"))
		return
	}
	if state.router == nil {
		t.setNoAvailableMenu()
		t.desk.SetSystemTrayIcon(GenerateIcon("---"))
		return
	}

	t.buildStateMenu(state)

	if state.activeIface != nil {
		label := PolicyLabel(state.activeIface.Policy, state.policies, state.activeIface.Deny)
		short := PolicyShort(label)
		t.desk.SetSystemTrayIcon(GenerateIcon(short))
	} else {
		t.desk.SetSystemTrayIcon(GenerateIcon("---"))
	}
}

// --- Menu builders ---

func (t *TrayApp) setLoadingMenu() {
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Loading...", nil),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Refresh", t.scheduleRefresh),
		fyne.NewMenuItem("Quit", t.app.Quit),
	)
	menu.Items[0].Disabled = true
	t.desk.SetSystemTrayMenu(menu)
}

func (t *TrayApp) setNoRoutersMenu() {
	menu := fyne.NewMenu("",
		disabledItem("No routers configured."),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Add Router...", t.openSettingsAdd),
		fyne.NewMenuItem("Quit", t.app.Quit),
	)
	t.desk.SetSystemTrayMenu(menu)
}

func (t *TrayApp) setNoAvailableMenu() {
	menu := fyne.NewMenu("",
		disabledItem("No routers available on this network."),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Add Router...", t.openSettingsAdd),
		fyne.NewMenuItem("Settings...", t.openSettings),
		fyne.NewMenuItem("Refresh", t.scheduleRefresh),
		fyne.NewMenuItem("Quit", t.app.Quit),
	)
	t.desk.SetSystemTrayMenu(menu)
}

func (t *TrayApp) buildStateMenu(state *appState) {
	var items []*fyne.MenuItem

	// Inline entries for active interface
	if state.activeIface != nil {
		items = append(items, t.buildIfaceItems(state.activeIface, state.policies)...)
		items = append(items, fyne.NewMenuItemSeparator())
	}

	// Submenus for additional interfaces
	for i := range state.interfaces {
		iface := &state.interfaces[i]
		if state.activeIface != nil && iface.Name == state.activeIface.Name {
			continue
		}
		sub := fyne.NewMenu(iface.DisplayName, t.buildIfaceItems(iface, state.policies)...)
		submenuItem := fyne.NewMenuItem(iface.DisplayName, nil)
		submenuItem.ChildMenu = sub
		items = append(items, submenuItem)
	}

	if state.routerInfo != nil {
		items = append(items, fyne.NewMenuItemSeparator())
		items = append(items, disabledItem("Router: "+state.routerInfo.Name))
	}

	items = append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Settings...", t.openSettings),
		fyne.NewMenuItem("Refresh", t.scheduleRefresh),
		fyne.NewMenuItem("Quit", t.app.Quit),
	)

	t.desk.SetSystemTrayMenu(fyne.NewMenu("", items...))
}

func (t *TrayApp) buildIfaceItems(iface *InterfaceInfo, policies map[string]interface{}) []*fyne.MenuItem {
	state := "Offline"
	if iface.Online {
		state = "Online"
	}

	currentLabel := PolicyLabel(iface.Policy, policies, iface.Deny)

	var items []*fyne.MenuItem
	items = append(items,
		disabledItem(iface.DisplayName),
		disabledItem("IP: "+iface.IP),
		disabledItem("MAC: "+iface.MAC),
		disabledItem("Type: "+iface.IFType),
		disabledItem("State: "+state),
		fyne.NewMenuItemSeparator(),
	)

	mac := iface.MAC

	def := fyne.NewMenuItem("Default", func() { t.applyPolicy(mac, "", "default") })
	def.Checked = currentLabel == "Default"
	items = append(items, def)

	blocked := fyne.NewMenuItem("Blocked", func() { t.applyPolicy(mac, "", "blocked") })
	blocked.Checked = currentLabel == "Blocked"
	items = append(items, blocked)

	for policyName, info := range policies {
		title := policyName
		if m, ok := info.(map[string]interface{}); ok {
			if desc, ok := m["description"].(string); ok && desc != "" {
				title = desc
			}
		}
		pn := policyName
		item := fyne.NewMenuItem(title, func() { t.applyPolicy(mac, pn, "policy") })
		item.Checked = currentLabel == title
		items = append(items, item)
	}

	return items
}

func (t *TrayApp) applyPolicy(mac, policyName, mode string) {
	state := t.state
	if state == nil || state.router == nil {
		return
	}
	router := state.router

	go func() {
		var err error
		switch mode {
		case "blocked":
			err = router.SetClientBlock(mac)
		case "default":
			err = router.ApplyPolicy(mac, "")
		default:
			err = router.ApplyPolicy(mac, policyName)
		}
		if err != nil {
			t.app.SendNotification(&fyne.Notification{
				Title:   "Keenetic Tray",
				Content: "Failed to apply policy: " + err.Error(),
			})
		}
		t.scheduleRefresh()
	}()
}

func (t *TrayApp) openSettings() {
	t.mu.Lock()
	routers := make([]RouterConfig, len(t.routers))
	copy(routers, t.routers)
	t.mu.Unlock()

	showSettingsWindow(t.app, routers, func(updated []RouterConfig) {
		t.mu.Lock()
		t.routers = updated
		t.mu.Unlock()
		_ = saveRouters(updated)
		t.scheduleRefresh()
	})
}

func (t *TrayApp) openSettingsAdd() {
	t.mu.Lock()
	routers := make([]RouterConfig, len(t.routers))
	copy(routers, t.routers)
	t.mu.Unlock()

	showAddRouterDialog(t.app, routers, func(updated []RouterConfig) {
		t.mu.Lock()
		t.routers = updated
		t.mu.Unlock()
		_ = saveRouters(updated)
		t.scheduleRefresh()
	})
}

func disabledItem(label string) *fyne.MenuItem {
	item := fyne.NewMenuItem(label, nil)
	item.Disabled = true
	return item
}
