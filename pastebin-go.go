package main

import (
    "log"
    "net/http"
    "html/template"
)

var templates = template.Must(template.ParseFiles("html/index.html"))

func indexHandler(w http.ResponseWriter, r *http.Request) {
    _ = templates.ExecuteTemplate(w, "index.html", nil)
}

func main() {
    http.HandleFunc("/", indexHandler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
