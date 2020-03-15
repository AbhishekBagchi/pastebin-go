package main

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var templates = template.Must(template.ParseGlob(filepath.Join(os.Getenv("TEMPLATE_DIR"), "*.html")))
var hash = sha256.New()

type Paste struct {
	Link     template.URL
	Contents string
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		err := templates.ExecuteTemplate(w, "indexPage", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		data := []byte(r.FormValue("contents"))
		hash := hash.Sum(data)
		filename := hex.EncodeToString(hash[0:8])

		//FIXME First implimentation. Can and should be improved.
		err := ioutil.WriteFile(filename, data, 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/view/"+filename, http.StatusSeeOther)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	idx := strings.Index(urlPath, "/view/")
	if idx == -1 {
		http.Error(w, "Invalid URL format", http.StatusInternalServerError)
		return
	}
	link := template.URL("http://" + r.Host + urlPath)
	filename := urlPath[6:]
	_, err := os.Stat(filename)
	if err != nil {
		http.Redirect(w, r, "/error/", http.StatusSeeOther)
		return
	}
	dat, err := ioutil.ReadFile(filename)
	p := &Paste{Link: link, Contents: string(dat)}
	err = templates.ExecuteTemplate(w, "viewPage", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "errorPage", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	//Serve static CSS etc
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(os.Getenv("STATIC_DIR")))))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/error/", errorHandler)
	http.HandleFunc("/view/", viewHandler)
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
