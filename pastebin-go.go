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
//    "time"
)

var templates = template.Must(template.ParseGlob(filepath.Join(os.Getenv("TEMPLATE_DIR"), "*.html")))
var hash = sha256.New()

func pasteHandler(w http.ResponseWriter, r *http.Request) {
//    t0 := time.Now()
    data := []byte(r.FormValue("contents"))
//    t1 := time.Now()
//    log.Printf("Get Contenst and cast to byte %v", t1.Sub(t0))
    hash := hash.Sum(data)
//    t3 := time.Now()
//    log.Printf("Hash sum %v", t3.Sub(t1))
    filename := hex.EncodeToString(hash[0:8])
//    t4 := time.Now()
//    log.Printf("Hex encode %v", t4.Sub(t3))
//    log.Printf("Total %v", t4.Sub(t0))

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
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/paste", pasteHandler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
