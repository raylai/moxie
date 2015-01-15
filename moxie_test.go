package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestConnect(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8080/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status Code %d", resp.StatusCode)
	}
}

func TestListRoot(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8080/")
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

func TestPut(t *testing.T) {
	const FILE = "moxie_test.go"
	const BASE = "http://127.0.0.1:8080/"

	for _, dir := range []string{"", "d/", "a/b/c/"} {
		url := BASE + dir + FILE

		// Read test file
		file, err := os.Open(FILE)
		if err != nil {
			t.Fatal(err)
		}
		client := &http.Client{}
		filebody, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}

		// Upload file
		preq, err := http.NewRequest("PUT", url, file)
		if err != nil {
			t.Fatal(err)
		}
		presp, err := client.Do(preq)
		if err != nil {
			t.Fatal(err)
		}
		defer presp.Body.Close()
		if presp.StatusCode != http.StatusOK {
			t.Fatalf("Status Code %d", presp.StatusCode)
		}

		// Get file
		gresp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer gresp.Body.Close()
		if gresp.StatusCode != http.StatusOK {
			t.Fatalf("Status Code %d", gresp.StatusCode)
		}
		body, err := ioutil.ReadAll(gresp.Body)
		if err != nil {
			t.Fatal(err)
		}

		// Compare
		if string(filebody) != string(body) {
			t.Fatal("PUT failed")
		}
	}
}
