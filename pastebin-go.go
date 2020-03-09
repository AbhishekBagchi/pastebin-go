package main

import (
    "log"
    "net/http"
    "html/template"
)

var templates = template.Must(template.ParseGlob("html/*.html"))

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
