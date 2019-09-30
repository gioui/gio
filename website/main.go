// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	http.HandleFunc("/issue/", issueHandler)
	http.HandleFunc("/commit/", commitHandler)
	http.HandleFunc("/patches/", patchesHandler)
	http.HandleFunc("/", vanityHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func patchesHandler(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	const pref = "/patches"
	url := "https://lists.sr.ht/~eliasnaur/gio/patches" + p[len(pref):]
	http.Redirect(w, r, url, http.StatusFound)
}

func issueHandler(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	const pref = "/issue"
	url := "https://todo.sr.ht/~eliasnaur/gio" + p[len(pref):]
	http.Redirect(w, r, url, http.StatusFound)
}

func commitHandler(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	const pref = "/commit"
	commit := p[len(pref):]
	var url string
	if commit == "/" {
		url = "https://git.sr.ht/~eliasnaur/gio/log"
	} else {
		url = "https://git.sr.ht/~eliasnaur/gio/commit" + commit
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// vanityHandler serves git location meta headers for the go tool.
func vanityHandler(w http.ResponseWriter, r *http.Request) {
	if www := "www."; strings.HasPrefix(r.URL.Host, www) {
		r.URL.Host = r.URL.Host[len(www):]
		http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently)
		return
	}
	if r.URL.Query().Get("go-get") == "1" {
		fmt.Fprintf(w, `<html><head>
<meta name="go-import" content="gioui.org git https://git.sr.ht/~eliasnaur/gio">
<meta name="go-source" content="gioui.org _ https://git.sr.ht/~eliasnaur/gio/tree/master{/dir} https://git.sr.ht/~eliasnaur/gio/tree/master{/dir}/{file}#L{line}">
</head></html>`)
		return
	}
	p := r.URL.Path
	switch {
	case p == "/":
		http.Redirect(w, r, "https://git.sr.ht/~eliasnaur/gio", http.StatusFound)
	default:
		http.Redirect(w, r, "https://godoc.org/gioui.org"+p, http.StatusFound)
	}
}
