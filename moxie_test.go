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

func TestConcurrentGet(t *testing.T) {
	const PATH = "/MySQL Dump/dump/location.txt.gz"
	const URL = "http://127.0.0.1:8080" + PATH
	const CACHEFILE = "cache" + PATH
	const PARTFILE = CACHEFILE + ".part"
	const MAX = 10

	// Clear cache
	if err := os.Remove(CACHEFILE); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Remove(PARTFILE); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	c := make(chan int)
	for i := 0; i < MAX; i++ {
		go func(n int) {
			resp, err := http.Get(URL)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			c <- n
		}(i)
	}

	for i := 0; i < MAX; i++ {
		<-c
	}
}
