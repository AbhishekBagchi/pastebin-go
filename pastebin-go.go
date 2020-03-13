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

func pasteHandler(w http.ResponseWriter, r *http.Request) {
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

func indexHandler(w http.ResponseWriter, r *http.Request) {
    err := templates.ExecuteTemplate(w, "indexPage", nil)
    if (err != nil) {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

func main() {
    //Serve static CSS etc
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(os.Getenv("STATIC_DIR")))))

    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/paste", pasteHandler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
