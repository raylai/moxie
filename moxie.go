package main

import (
	"fmt"
	"github.com/t3rm1n4l/go-mega"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

const CACHEDIR = "cache"
var megaSession *mega.Mega

func handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		node, err := node(r.URL.Path)
		if err != nil {
			// XXX 404
			log.Print(err)
			return
		}
		switch node.GetType() {
		case mega.FOLDER, mega.ROOT:
			list(w, r, node)
		default:
			get(w, r, node)
		}

	case "PUT":
		put(w, r)
	}
}

func list(w http.ResponseWriter, r *http.Request, node *mega.Node) {
	children, err := megaSession.FS.GetChildren(node)
	if err != nil {
		log.Print(err)
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
		log.Print(err)
		os.Remove(tempfile)
		return
	}
	file, err := os.Open(tempfile) // For read access.
	if err != nil {
		log.Print(err)
		return
	}
	defer file.Close()
	_, err = io.Copy(w, file)
	if err != nil {
		log.Print(err)
		return
	}
}

func put(w http.ResponseWriter, r *http.Request) {
	cachefile := CACHEDIR + r.URL.Path
	dir, name := path.Split(cachefile)

	// Create local file
	if err := os.MkdirAll(dir, 0700); err != nil && !os.IsExist(err) {
		log.Print(err)
		return
	}
	fp, err := os.Create(cachefile)
	if err != nil {
		log.Print(err)
		return
	}
	_, err = io.Copy(fp, r.Body)
	if err != nil {
		log.Print(err)
		return
	}

	// Create Mega path
	dirarray := strings.Split(r.URL.Path, "/")
	root := megaSession.FS.GetRoot()
	n, err := mkpath(dirarray[1:len(dirarray)-1], root)
	if err != nil {
		log.Print(err)
		return
	}

	// Lookup Mega file (if it exists)
	paths, err := megaSession.FS.PathLookup(root, dirarray[1:])
	// Log unexpected errors
	if err != nil && err.Error() != "Object (typically, node or user) not found" {
		log.Print(err)
		return
	}
	// File exists, so delete it before uploading new file
	if err == nil {
		// We only care about the last node.
		lastnode := paths[len(paths)-1]
		// File exists, delete! (aka overwrite)
		if err = megaSession.Delete(lastnode, false); err != nil {
			log.Print(err)
			return
		}
	}
	// Finally, upload file
	_, err = megaSession.UploadFile(cachefile, n, name, nil)
	if err != nil {
		log.Print(err)
		return
	}
}

func mkpath(p []string, parent *mega.Node) (*mega.Node, error) {
	var n *mega.Node
	var err error

	root := megaSession.FS.GetRoot()
	// Root path is an empty array.
	if p[0] == "" {
		return root, nil
	}

	paths, err := megaSession.FS.PathLookup(root, p)
	if err == nil {
		// We only care about the last path.
		return paths[len(paths)-1], nil
	} else if err.Error() != "Object (typically, node or user) not found" {
		log.Printf("not exist: %#v\n", err)
		return nil, err
	}

	l := len(p)
	if l == 1{
		n = parent
	} else {
		// if a/b/c then parent = mkpath(a/b)
		n, err = mkpath(p[:l-1], parent)
		if err != nil {
			return n, err
		}
	}
	return megaSession.CreateDir(p[l-1], n)
}

func node(url string) (*mega.Node, error) {
	trimmedPath := strings.Trim(url, "/")
	path := strings.Split(trimmedPath, "/")
	root := megaSession.FS.GetRoot()
	// Root path is an empty array.
	if path[0] == "" {
		return root, nil
	} else {
		paths, err := megaSession.FS.PathLookup(root, path)
		if err != nil {
			// XXX should be 404
			return nil, err
		}
		// We only care about the last path.
		return paths[len(paths)-1], nil
	}
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
