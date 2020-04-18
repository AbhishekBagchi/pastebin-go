package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"github.com/AbhishekBagchi/kvdb"
	"html/template"
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
var static_dir string
var db_filename string
var database *kvdb.Database
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
		hash := sha256.Sum256(data)
		key := hex.EncodeToString(hash[0:8])
		log.Printf("%s: %s", key, data)

		db_err := database.Insert(key, data, false)
		if db_err != nil {
			log.Printf(db_err.String())
			http.Error(w, "Server error", 501)
			return
		}
		defer database.Export(db_filename)

		http.Redirect(w, r, "/view/"+key, http.StatusSeeOther)
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
	key := urlPath[6:]
	dat, db_err := database.Get(key)
	if db_err != nil {
		log.Printf(db_err.String())
		http.Error(w, "Server error", 501)
		return
	}
	p := &Paste{Link: link, Contents: string(dat)}
	err := templates.ExecuteTemplate(w, "viewPage", p)
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

func init_global(template_flag *string, static_flag *string, db_flag *string) {
	template_dir := *template_flag
	if template_dir == "" {
		template_dir = os.Getenv("TEMPLATE_DIR")
	}
	templates = template.Must(template.ParseGlob(filepath.Join(template_dir, "*.html")))

	static_dir = *static_flag
	if static_dir == "" {
		static_dir = os.Getenv("STATIC_DIR")
	}

	var err error
	db_filename = *db_flag
	database, err = kvdb.Open(db_filename, true)
	if err != nil {
		panic(err)
	}
}

func main() {
	var addrs listenAddresses
	flag.Var(&addrs, "interface", "Interface to listen to. Interfaces are of the form <address>:<port>. Call multiple times for multiple addresses")
	tmpl_flag := flag.String("template-dir", "", "Directory for template files")
	static_flag := flag.String("static-dir", "", "Directory for static files")
	dbfile_flag := flag.String("database", "paste.kvdb", "Filename to store the database in")
	flag.Parse()

	init_global(tmpl_flag, static_flag, dbfile_flag)
	defer database.Export(db_filename)

	if len(addrs) == 0 {
		addrs = append(addrs, "localhost:8080")
	}
	checkIfLowPort(addrs)

	var wg sync.WaitGroup
	for _, ifc := range addrs {
		wg.Add(1)
		go startServer(&wg, static_dir, ifc)
	}
	wg.Wait()
}
