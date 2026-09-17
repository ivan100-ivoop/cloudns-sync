package cloudns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultAPIURL = "https://api.cloudns.net"

type Client struct {
	APIURL       string
	AuthID       string
	AuthPassword string
	HTTPClient   *http.Client
}

type RegisterResponse struct {
	Status            string `json:"status"`
	StatusDescription string `json:"statusDescription"`
}

type zoneListItem struct {
	Name       string `json:"name"`
	DomainName string `json:"domain-name"`
	Domain     string `json:"domain"`
	ZoneType   string `json:"zone-type"`
	Type       string `json:"type"`
}

type Zone struct {
	Name string
	Type string
}

func (client Client) ListZones(ctx context.Context) (map[string]Zone, error) {
	endpoint := client.endpoint("list-zones.json")
	result := make(map[string]Zone)
	for page := 1; ; page++ {
		form := url.Values{
			"auth-id":       {client.AuthID},
			"auth-password": {client.AuthPassword},
			"page":          {fmt.Sprint(page)},
			"rows-per-page": {"100"},
		}
		body, err := client.postForm(ctx, endpoint, form)
		if err != nil {
			return nil, err
		}
		items, err := decodeZoneList(body)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			name := item.Name
			if name == "" {
				name = item.DomainName
			}
			if name == "" {
				name = item.Domain
			}
			name = normalizeZone(name)
			if name != "" {
				zoneType := item.ZoneType
				if zoneType == "" {
					zoneType = item.Type
				}
				result[name] = Zone{Name: name, Type: strings.ToLower(strings.TrimSpace(zoneType))}
			}
		}
		if len(items) < 100 {
			return result, nil
		}
	}
}

func (client Client) RegisterSlaveZone(ctx context.Context, zone string, masterIP string) error {
	form := url.Values{
		"auth-id":       {client.AuthID},
		"auth-password": {client.AuthPassword},
		"domain-name":   {zone},
		"zone-type":     {"slave"},
		"master-ip":     {masterIP},
	}
	body, err := client.postForm(ctx, client.endpoint("register.json"), form)
	if err != nil {
		return err
	}
	var result RegisterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode ClouDNS response: %w", err)
	}
	if strings.ToLower(result.Status) != "success" && result.Status != "1" {
		return fmt.Errorf("ClouDNS rejected slave zone %q: %s", zone, result.StatusDescription)
	}
	return nil
}

func (client Client) DeleteZone(ctx context.Context, zone string) error {
	form := url.Values{
		"auth-id":       {client.AuthID},
		"auth-password": {client.AuthPassword},
		"domain-name":   {zone},
	}
	body, err := client.postForm(ctx, client.endpoint("delete.json"), form)
	if err != nil {
		return err
	}
	var result RegisterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode ClouDNS response: %w", err)
	}
	if strings.ToLower(result.Status) != "success" && result.Status != "1" {
		return fmt.Errorf("ClouDNS rejected deletion of zone %q: %s", zone, result.StatusDescription)
	}
	return nil
}

// endpoint resolves an API method against APIURL. APIURL is normally a base
// URL, but full method URLs remain supported for backward compatibility.
func (client Client) endpoint(method string) string {
	base := strings.TrimRight(strings.TrimSpace(client.APIURL), "/")
	if base == "" {
		base = defaultAPIURL
	}

	for _, knownMethod := range []string{"register.json", "list-zones.json", "delete.json"} {
		suffix := "/" + knownMethod
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, knownMethod) + method
		}
	}

	if strings.HasSuffix(base, "/dns") {
		return base + "/" + method
	}
	return base + "/dns/" + method
}

func (client Client) postForm(ctx context.Context, endpoint string, form url.Values) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create ClouDNS request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send ClouDNS request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read ClouDNS response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("ClouDNS returned HTTP %d", response.StatusCode)
	}
	return body, nil
}

func decodeZoneList(body []byte) ([]zoneListItem, error) {
	var items []zoneListItem
	if err := json.Unmarshal(body, &items); err == nil {
		return items, nil
	}
	var envelope struct {
		Data []zoneListItem `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Data != nil {
		return envelope.Data, nil
	}
	var status RegisterResponse
	if err := json.Unmarshal(body, &status); err == nil && status.Status != "" {
		return nil, fmt.Errorf("ClouDNS list zones failed: %s", status.StatusDescription)
	}
	return nil, fmt.Errorf("decode ClouDNS zone list response")
}

func normalizeZone(zone string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(zone)), ".")
}
