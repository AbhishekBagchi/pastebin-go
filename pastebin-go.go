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
)

var templates = template.Must(template.ParseGlob(filepath.Join(os.Getenv("TEMPLATE_DIR"), "*.html")))
var hash = sha256.New()

func indexHandler(w http.ResponseWriter, r *http.Request) {
    if (r.Method == http.MethodGet) {
        err := templates.ExecuteTemplate(w, "indexPage", nil)
        if (err != nil) {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
    } else if (r.Method == http.MethodPost) {
        data := []byte(r.FormValue("contents"))
        hash := hash.Sum(data)
        filename := hex.EncodeToString(hash[0:8])

        //FIXME First implimentation. Can and should be improved.
        err := ioutil.WriteFile(filename, data, 0644)
        if (err != nil) {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
    }
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
    err := templates.ExecuteTemplate(w, "errorPage", nil)
    if (err != nil) {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

func main() {
    //Serve static CSS etc
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(os.Getenv("STATIC_DIR")))))

    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/error", errorHandler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
