package moxie

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestConnect(t *testing.T) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", "http://127.0.0.1:8080", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status Code %d", resp.StatusCode)
	}
}

func TestListRoot(t *testing.T) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", "http://127.0.0.1:8080", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status Code %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html><body><h1>/</h1><ul>") {
		t.Fatal("listing failed")
	}
	if !strings.Contains(string(body), "</ul></body></html>") {
		t.Fatal("listing failed")
	}
}
