package main

import (
	"net"
	"strings"

	"github.com/jackpal/gateway"
)

// InterfaceInfo describes a local network interface matched to a router client.
type InterfaceInfo struct {
	Name        string
	DisplayName string
	MAC         string
	IP          string
	IFType      string
	Online      bool
	Policy      string
	Deny        bool
}

func getLocalNetworks() []*net.IPNet {
	var networks []*net.IPNet
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					networks = append(networks, ipnet)
				}
			}
		}
	}
	return networks
}

func isIPInNetworks(ipStr string, networks []*net.IPNet) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func extractHost(address string) string {
	if address == "" {
		return ""
	}
	if idx := strings.Index(address, "://"); idx >= 0 {
		address = address[idx+3:]
	}
	if idx := strings.Index(address, "/"); idx >= 0 {
		address = address[:idx]
	}
	return address
}

func getDefaultInterfaceName() string {
	localIP, err := gateway.DiscoverInterface()
	if err != nil {
		return ""
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.Equal(localIP) {
				return iface.Name
			}
		}
	}
	return ""
}

func guessIFType(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "wl") || strings.HasPrefix(lower, "wlan") || strings.HasPrefix(lower, "wifi"):
		return "Wi-Fi"
	case strings.HasPrefix(lower, "en") || strings.HasPrefix(lower, "eth"):
		return "Ethernet"
	default:
		return "Unknown"
	}
}

func listLocalInterfaces(clients []Client) []InterfaceInfo {
	clientsByMAC := make(map[string]Client)
	for _, c := range clients {
		clientsByMAC[strings.ToLower(c.MAC)] = c
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var result []InterfaceInfo
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		mac := strings.ToLower(iface.HardwareAddr.String())
		if mac == "" {
			continue
		}
		cl, matched := clientsByMAC[mac]
		if !matched {
			continue
		}

		ip := "N/A"
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					ip = ip4.String()
					break
				}
			}
		}

		displayName := iface.Name
		if cl.Name != "" && cl.Name != "Unknown" {
			displayName = cl.Name
		}

		online := false
		if data := cl.Data; data != nil {
			if link, ok := data["link"].(string); ok && link == "up" {
				online = true
			}
			if mws, ok := data["mws"].(map[string]interface{}); ok {
				if link, ok := mws["link"].(string); ok && link == "up" {
					online = true
				}
			}
		}

		result = append(result, InterfaceInfo{
			Name:        iface.Name,
			DisplayName: displayName,
			MAC:         mac,
			IP:          ip,
			IFType:      guessIFType(iface.Name),
			Online:      online,
			Policy:      cl.Policy,
			Deny:        cl.Deny,
		})
	}
	return result
}
