package cloudns

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRegisterSlaveZone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		body, _ := io.ReadAll(request.Body)
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("zone-type") != "slave" || values.Get("domain-name") != "example.com" || values.Get("master-ip") != "192.0.2.10" {
			t.Fatalf("unexpected form: %v", values)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":"Success","statusDescription":"Zone registered"}`))
	}))
	defer server.Close()

	client := Client{APIURL: server.URL, AuthID: "123", AuthPassword: "secret"}
	if err := client.RegisterSlaveZone(context.Background(), "example.com", "192.0.2.10"); err != nil {
		t.Fatal(err)
	}
}

func TestListZones(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`[{"name":"Example.COM."},{"domain-name":"existing.net"}]`))
	}))
	defer server.Close()
	client := Client{APIURL: server.URL + "/register.json", AuthID: "123", AuthPassword: "secret"}
	zones, err := client.ListZones(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zones["example.com"]; !ok {
		t.Fatalf("missing normalized zone: %#v", zones)
	}
	if _, ok := zones["existing.net"]; !ok {
		t.Fatalf("missing domain-name zone: %#v", zones)
	}
}

func TestDeleteZone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		if values.Get("domain-name") != "gone.example" {
			t.Fatalf("unexpected delete form: %v", values)
		}
		_, _ = response.Write([]byte(`{"status":"Success","statusDescription":"Zone deleted"}`))
	}))
	defer server.Close()
	client := Client{APIURL: server.URL + "/register.json", AuthID: "123", AuthPassword: "secret"}
	if err := client.DeleteZone(context.Background(), "gone.example"); err != nil {
		t.Fatal(err)
	}
}
