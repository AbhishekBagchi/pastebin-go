package main

import (
    "html/template"
    "log"
    "net/http"
    "os"
    "path/filepath"
)

var templates = template.Must(template.ParseGlob(filepath.Join(os.Getenv("TEMPLATE_DIR"), "*.html")))

func indexHandler(w http.ResponseWriter, r *http.Request) {
    err := templates.ExecuteTemplate(w, "indexPage", nil)
    if (err != nil) {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

func main() {
    http.HandleFunc("/", indexHandler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
