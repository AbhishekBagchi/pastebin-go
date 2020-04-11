package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

//Command line args

//Interface addresses. Passed in multiple times for multiple values
type listenAddresses []string

func (i *listenAddresses) String() string {
	str := ""
	for _, s := range *i {
		str += s
	}
	return str
}

func (i *listenAddresses) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var templates *template.Template
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

func startServer(wg *sync.WaitGroup, static_dir string, ifc string) {
	defer wg.Done()
	log.Printf("Listening on %s", ifc)
	serveMux := http.NewServeMux()
	//Serve static CSS etc
	serveMux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(static_dir))))
	serveMux.HandleFunc("/", indexHandler)
	serveMux.HandleFunc("/error/", errorHandler)
	serveMux.HandleFunc("/view/", viewHandler)
	log.Fatal(http.ListenAndServe(ifc, serveMux))
}

//checkIfLowPort checks if user needs to be root to bind to a certain port and panics if a non root user tries to do so
func checkIfLowPort(addrs listenAddresses) {
	for _, addr := range addrs {
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			log.Fatal("Invalid format: ", addr)
		}
		portNum, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Fatal("Invalid format: ", addr)
		}
		if portNum <= 1024 && os.Geteuid() != 0 {
			log.Fatal("Need to be root to bind to ", addr)
		}
	}
}

func main() {
	var addrs listenAddresses
	flag.Var(&addrs, "interface", "Interface to listen to. Interfaces are of the form <address>:<port>. Call multiple times for multiple addresses")
	tmpl_flag := flag.String("template-dir", "", "Directory for template files")
	static_flag := flag.String("static-dir", "", "Directory for static files")
	flag.Parse()

	if len(addrs) == 0 {
		addrs = append(addrs, "localhost:8080")
	}
	checkIfLowPort(addrs)

	template_dir := *tmpl_flag
	if template_dir == "" {
		template_dir = os.Getenv("TEMPLATE_DIR")
	}
	templates = template.Must(template.ParseGlob(filepath.Join(template_dir, "*.html")))
	static_dir := *static_flag
	if static_dir == "" {
		static_dir = os.Getenv("STATIC_DIR")
	}

	var wg sync.WaitGroup
	for _, ifc := range addrs {
		wg.Add(1)
		go startServer(&wg, static_dir, ifc)
	}
	wg.Wait()
}
