package main

import (
	"fmt"
	"github.com/t3rm1n4l/go-mega"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const CACHEDIR = "cache"
var megaSession *mega.Mega

func handle(w http.ResponseWriter, r *http.Request) {
	var node *mega.Node

	trimmedPath := strings.Trim(r.URL.Path, "/")
	path := strings.Split(trimmedPath, "/")
	root := megaSession.FS.GetRoot()
	// Root path is an empty array.
	if path[0] == "" {
		node = root
	} else {
		paths, err := megaSession.FS.PathLookup(root, path)
		if err != nil {
			// XXX should be 404
			fmt.Fprintf(w, "invalid path: %s\n", r.URL.Path)
			return
		}
		// We only care about the last path.
		node = paths[len(paths)-1]
	}

	switch r.Method {
	case "GET":
		switch node.GetType() {
		case mega.FOLDER, mega.ROOT:
			list(w, r, node)
		default:
			get(w, r, node)
		}

	case "PUT":
		put(w, r, node)
	}
}

func list(w http.ResponseWriter, r *http.Request, node *mega.Node) {
	type List struct {
		File []string
	}
	children, err := megaSession.FS.GetChildren(node)
	if err != nil {
		fmt.Fprintf(w, "error getting children: %s", err)
		// XXX 500
		return
	}

	fmt.Fprint(w, "<html><body>")
	fmt.Fprintf(w, "<h1>%s</h1>", html.EscapeString(r.URL.Path))
	fmt.Fprint(w, "<ul>")
	if node != megaSession.FS.GetRoot() {
		fmt.Fprintf(w, "<li><a href=\"..\">..</a>")
	}
	for _, child := range children {
		var folder string
		if child.GetType() == mega.FOLDER {
			folder = "/"
		}
		fmt.Fprintf(w, "<li><a href=\"%s%s\">%s%s</a>",
			html.EscapeString(child.GetName()), folder,
			html.EscapeString(child.GetName()), folder)
	}
	fmt.Fprint(w, "</ul></body></html>")
}

func get(w http.ResponseWriter, r *http.Request, node *mega.Node) {
	hash := node.GetHash()
	tempfile := fmt.Sprintf("%s/%s", CACHEDIR, hash)
	err := megaSession.DownloadFile(node, tempfile, nil)
	if err != nil {
		log.Printf("download failed: %s", err)
		os.Remove(tempfile)
		return
	}
	file, err := os.Open(tempfile) // For read access.
	if err != nil {
		log.Printf("error opening %s: %s", tempfile, err)
	}
	defer file.Close()
	data := make([]byte, 1024*1024)
	for {
		nr, err := file.Read(data)
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Printf("error reading %s: %s", tempfile, err)
		}
		for nr != 0 {
			nw, err := w.Write(data[:nr])
			if err != nil {
				log.Printf("error sending data: %s", err)
			}
			nr -= nw
		}
	}
}

func put(w http.ResponseWriter, r *http.Request, node *mega.Node) {
	//func (m *Mega) CreateDir(name string, parent *Node) (*Node, error)
	/*
		tmpfile = ok.MkTemp()
		defer os.Remove(tmpfile)
		write(tmpfile, r.Data)
		name := path[len(path)-1]
		megaSession.UploadFile(tmpfile, node, name, nil) (*Node, error)
	*/
}

func main() {
	user := os.Getenv("MEGA_USER")
	pass := os.Getenv("MEGA_PASSWD")
	megaSession = mega.New()
	if err := megaSession.Login(user, pass); err != nil {
		log.Fatal(err)
	}
	if err := os.Mkdir(CACHEDIR, 0700); err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
	http.HandleFunc("/", handle)
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
