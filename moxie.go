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
	"sync"
	"time"
)

const CACHEDIR = "cache"

var megaSession *mega.Mega
var mutex sync.Mutex

func handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		get(w, r)
	case "PUT":
		put(w, r)
	}
}

func list(w http.ResponseWriter, r *http.Request, node *mega.Node) {
	mutex.Lock()
	children, err := megaSession.FS.GetChildren(node)
	mutex.Unlock()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}

	fmt.Fprint(w, "<html><body>")
	fmt.Fprintf(w, "<h1>%s</h1>", html.EscapeString(r.URL.Path))
	fmt.Fprint(w, "<ul>")
	mutex.Lock()
	if node != megaSession.FS.GetRoot() {
		fmt.Fprintf(w, "<li><a href=\"..\">..</a>")
	}
	mutex.Unlock()
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

func get(w http.ResponseWriter, r *http.Request) {
	node, err := lookup(r.URL.Path)
	if err != nil {
		if err.Error() == "Object (typically, node or user) not found" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// List directories
	switch node.GetType() {
	case mega.FOLDER, mega.ROOT:
		list(w, r, node)
		return
	}

	// Cache files
	cachefile := CACHEDIR + r.URL.Path
	dir, _ := path.Split(cachefile)

	// Do we have this cached?
	file, err := os.Open(cachefile)
	if err != nil {
		// Unexpected error
		if !os.IsNotExist(err) {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Expected error: file not found = cache miss
		// Build directory structure first
		if err := os.MkdirAll(dir, 0700); err != nil && !os.IsExist(err) {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tmpfile := cachefile + ".part"
		// If we can exclusively create tmpfile, we are the first to download.
		tmpfileh, err := os.OpenFile(tmpfile, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			// Unknown error
			if !os.IsExist(err) {
				log.Print(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// tmpfile already exists: wait 1 hour to finish downloading
			downloading := true
			for s := 1; downloading && s < 60*60; s++ {
				_, err := os.Stat(tmpfile)
				if err != nil {
					// tmpfile gone, download finished (if all goes well!)
					if os.IsNotExist(err) {
						downloading = false
						break
					}
					// Otherwise, unexpected error
					log.Print(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				time.Sleep(1 * time.Second)
			}
			// Still downloading?
			if downloading {
				w.WriteHeader(http.StatusRequestTimeout)
				return
			}
		} else {
			defer tmpfileh.Close()
			// Download file
			mutex.Lock()
			err = megaSession.DownloadFile(node, tmpfile, nil)
			mutex.Unlock()
			if err != nil {
				log.Print(err)
				w.WriteHeader(http.StatusInternalServerError)
				// Remove incomplete cachefile, in case one was created
				os.Remove(tmpfile)
				return
			}

			if err = os.Rename(tmpfile, cachefile); err != nil {
				log.Print(err)
				w.WriteHeader(http.StatusInternalServerError)
				os.Remove(tmpfile)
				return
			}
		}

		// Open cached file
		file, err = os.Open(cachefile)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	defer file.Close()
	_, err = io.Copy(w, file)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func put(w http.ResponseWriter, r *http.Request) {
	cachefile := CACHEDIR + r.URL.Path
	dir, name := path.Split(cachefile)

	// Create local file
	if err := os.MkdirAll(dir, 0700); err != nil && !os.IsExist(err) {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fp, err := os.Create(cachefile)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = io.Copy(fp, r.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Create Mega path
	dirarray := strings.Split(r.URL.Path, "/")
	mutex.Lock()
	root := megaSession.FS.GetRoot()
	mutex.Unlock()
	var n *mega.Node
	if len(dirarray) == 2 {
		n = root
	} else {
		n, err = mkpath(dirarray[1:len(dirarray)-1], root)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// Lookup Mega file (if it exists)
	mutex.Lock()
	paths, err := megaSession.FS.PathLookup(root, dirarray[1:])
	mutex.Unlock()
	// Log unexpected errors
	if err != nil && err.Error() != "Object (typically, node or user) not found" {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// File exists, so delete it before uploading new file
	if err == nil {
		// We only care about the last node.
		lastnode := paths[len(paths)-1]
		// File exists, delete! (aka overwrite)
		mutex.Lock()
		err = megaSession.Delete(lastnode, false)
		mutex.Unlock()
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	// Finally, upload file
	mutex.Lock()
	_, err = megaSession.UploadFile(cachefile, n, name, nil)
	mutex.Unlock()
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func mkpath(p []string, parent *mega.Node) (*mega.Node, error) {
	var n *mega.Node
	var err error

	mutex.Lock()
	root := megaSession.FS.GetRoot()
	mutex.Unlock()
	// Root path is an empty array.
	if p[0] == "" {
		return root, nil
	}

	mutex.Lock()
	paths, err := megaSession.FS.PathLookup(root, p)
	mutex.Unlock()
	// Path found
	if err == nil {
		// We only care about the last path.
		return paths[len(paths)-1], nil
	} else if err.Error() != "Object (typically, node or user) not found" {
		// Expected "not found" error, got something else
		return nil, err
	}

	l := len(p)
	if l == 1 {
		n = parent
	} else {
		// if a/b/c then parent = mkpath(a/b)
		n, err = mkpath(p[:l-1], parent)
		if err != nil {
			return n, err
		}
	}
	mutex.Lock()
	ret, err := megaSession.CreateDir(p[l-1], n)
	mutex.Unlock()
	return ret, err
}

func lookup(url string) (*mega.Node, error) {
	trimmedPath := strings.Trim(url, "/")
	path := strings.Split(trimmedPath, "/")
	mutex.Lock()
	root := megaSession.FS.GetRoot()
	mutex.Unlock()
	// Root path is an empty array.
	if path[0] == "" {
		return root, nil
	} else {
		mutex.Lock()
		paths, err := megaSession.FS.PathLookup(root, path)
		mutex.Unlock()
		if err != nil {
			return nil, err
		}
		// We only care about the last path.
		return paths[len(paths)-1], nil
	}
}

func main() {
	log.SetFlags(log.Lshortfile)
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
