package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

// Client represents a connected hotspot client.
type Client struct {
	Name   string                 `json:"name"`
	IP     string                 `json:"ip"`
	MAC    string                 `json:"mac"`
	Policy string                 `json:"policy"`
	Access string                 `json:"access"`
	Permit bool                   `json:"permit"`
	Deny   bool                   `json:"deny"`
	Online bool                   `json:"-"`
	Data   map[string]interface{} `json:"-"`
}

// KeeneticRouter communicates with the Keenetic router API.
type KeeneticRouter struct {
	BaseURL  string
	Username string
	Password string
	Name     string
	client   *http.Client
}

func NewKeeneticRouter(address, username, password, name string) *KeeneticRouter {
	if !strings.HasPrefix(address, "http") {
		address = strings.TrimSuffix(address, "/")
		address = "http://" + address
	}
	jar, _ := cookiejar.New(nil)
	return &KeeneticRouter{
		BaseURL:  address,
		Username: username,
		Password: password,
		Name:     name,
		client: &http.Client{
			Jar:     jar,
			Timeout: 10 * time.Second,
		},
	}
}

func (r *KeeneticRouter) Login() error {
	authURL := r.BaseURL + "/auth"

	resp, err := r.client.Get(authURL)
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}
	if resp.StatusCode != 401 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	realm := resp.Header.Get("X-NDM-Realm")
	challenge := resp.Header.Get("X-NDM-Challenge")

	md5sum := md5.Sum([]byte(r.Username + ":" + realm + ":" + r.Password))
	md5hex := fmt.Sprintf("%x", md5sum)

	sha256sum := sha256.Sum256([]byte(challenge + md5hex))
	sha256hex := fmt.Sprintf("%x", sha256sum)

	authData := map[string]string{"login": r.Username, "password": sha256hex}
	body, _ := json.Marshal(authData)

	authResp, err := r.client.Post(authURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("auth request error: %w", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != 200 {
		return fmt.Errorf("authentication failed: status %d", authResp.StatusCode)
	}
	return nil
}

func (r *KeeneticRouter) doRequest(endpoint string, data interface{}) ([]byte, error) {
	url := r.BaseURL + "/" + endpoint

	var resp *http.Response
	var err error

	if data != nil {
		body, _ := json.Marshal(data)
		resp, err = r.client.Post(url, "application/json", bytes.NewReader(body))
	} else {
		resp, err = r.client.Get(url)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (r *KeeneticRouter) GetNetworkIP() (string, error) {
	if err := r.Login(); err != nil {
		return "", err
	}
	data, err := r.doRequest("rci/sc/interface/Bridge0/ip/address", nil)
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	ip, _ := result["address"].(string)
	return ip, nil
}

func (r *KeeneticRouter) GetKeenDNSURLs() ([]string, error) {
	if err := r.Login(); err != nil {
		return nil, err
	}
	data, err := r.doRequest("rci/ip/http/ssl/acme/list/certificate", nil)
	if err != nil {
		return nil, err
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	var urls []string
	for _, item := range list {
		if d, ok := item["domain"].(string); ok {
			urls = append(urls, d)
		}
	}
	return urls, nil
}

func (r *KeeneticRouter) GetPolicies() (map[string]interface{}, error) {
	if err := r.Login(); err != nil {
		return nil, err
	}
	data, err := r.doRequest("rci/show/rc/ip/policy", nil)
	if err != nil {
		return nil, err
	}
	var policies map[string]interface{}
	if err := json.Unmarshal(data, &policies); err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *KeeneticRouter) GetOnlineClients() ([]Client, error) {
	if err := r.Login(); err != nil {
		return nil, err
	}

	// Fetch hotspot clients
	raw, err := r.doRequest("rci/show/ip/hotspot/host", nil)
	if err != nil {
		return nil, err
	}
	var rawClients []map[string]interface{}
	if err := json.Unmarshal(raw, &rawClients); err != nil {
		return nil, err
	}

	clientsByMAC := make(map[string]*Client)
	for _, c := range rawClients {
		mac := strings.ToLower(fmt.Sprintf("%v", c["mac"]))
		cl := &Client{
			MAC:    mac,
			Online: true,
			Data:   c,
		}
		if v, ok := c["name"].(string); ok {
			cl.Name = v
		}
		if v, ok := c["ip"].(string); ok {
			cl.IP = v
		}
		clientsByMAC[mac] = cl
	}

	// Fetch policy assignments
	policyRaw, err := r.doRequest("rci/show/rc/ip/hotspot/host", nil)
	if err == nil {
		var policyList []map[string]interface{}
		if json.Unmarshal(policyRaw, &policyList) == nil {
			for _, p := range policyList {
				mac := strings.ToLower(fmt.Sprintf("%v", p["mac"]))
				cl, exists := clientsByMAC[mac]
				if !exists {
					cl = &Client{MAC: mac, Name: "Unknown", IP: "N/A"}
					clientsByMAC[mac] = cl
				}
				if v, ok := p["policy"].(string); ok {
					cl.Policy = v
				}
				if v, ok := p["access"].(string); ok {
					cl.Access = v
				}
				if v, ok := p["permit"].(bool); ok {
					cl.Permit = v
				}
				if v, ok := p["deny"].(bool); ok {
					cl.Deny = v
				}
			}
		}
	}

	clients := make([]Client, 0, len(clientsByMAC))
	for _, cl := range clientsByMAC {
		clients = append(clients, *cl)
	}
	return clients, nil
}

func (r *KeeneticRouter) ApplyPolicy(mac, policy string) error {
	if err := r.Login(); err != nil {
		return err
	}
	var policyVal interface{} = policy
	if policy == "" {
		policyVal = false
	}
	data := map[string]interface{}{
		"mac":      mac,
		"policy":   policyVal,
		"permit":   true,
		"schedule": false,
	}
	_, err := r.doRequest("rci/ip/hotspot/host", data)
	return err
}

func (r *KeeneticRouter) SetClientBlock(mac string) error {
	if err := r.Login(); err != nil {
		return err
	}
	data := map[string]interface{}{
		"mac":      mac,
		"schedule": false,
		"deny":     true,
	}
	_, err := r.doRequest("rci/ip/hotspot/host", data)
	return err
}
