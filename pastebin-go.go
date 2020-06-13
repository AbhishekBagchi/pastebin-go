package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
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
	"time"
)

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

var config struct {
	templates   *template.Template
	static_dir  string
	db_filename string
	database    *kvdb.Database
}

var hash = sha256.New()

type paste struct {
	Link     template.URL
	Contents string
}

//encodeTime takes in the raw data being pasted and prepares it for storage in the database.
//The data is stored as <exprity_time uint64> + <paste_data>
//expiry_time is current_time + TTL. TTL is in minutes
func encodeTime(data []byte, ttl time.Duration) []byte {
	//Pre-prend a canary for some robustness
	canary := []byte{0xde, 0xad}
	if ttl == 0 {
		timeBytes := make([]byte, 8)
		for i := range timeBytes {
			timeBytes[i] = 0xFF
		}
		prepend := append(canary, timeBytes...)
		data = append(prepend, data...)
	} else {
		currTime := time.Now()
		expiryTime := (uint64)(currTime.Add(ttl).Unix())
		timeBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(timeBytes, expiryTime)
		prepend := append(canary, timeBytes...)
		data = append(prepend, data...)
	}
	return append(canary, data...)
}

//decodeTime gets the expiry time for a data chunk from the first 8 bytes
func decodeTime(data []byte) ([]byte, uint64, error) {
	canary := []byte{0xde, 0xad}
	if data[0] != canary[0] || data[1] != canary[1] {
		return nil, 0, errors.New("Malformed data")
	}
	timeBytes := data[2:10]
	return data[10:], binary.LittleEndian.Uint64(timeBytes), nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		err := config.templates.ExecuteTemplate(w, "indexPage", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		data := []byte(r.FormValue("contents"))
		ttl_unit := r.FormValue("ttl_unit")
		ttl_form := r.FormValue("ttl")
		var ttl time.Duration = 0
		if ttl_form != "" {
			ttl_value, err := strconv.Atoi(ttl_form)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			switch ttl_unit {
			case "minute":
				ttl = time.Duration(ttl_value) * time.Minute
			case "hour":
				ttl = time.Duration(ttl_value) * time.Hour
			case "day":
				ttl = time.Duration(ttl_value) * 24 * time.Hour
			}
		}
		data = encodeTime(data, ttl)
		hash := sha256.Sum256(data)
		key := hex.EncodeToString(hash[0:8])
		log.Printf("%s: %s, %v, %v", key, data)

		db_err := config.database.Insert(key, data, false)
		if db_err != nil {
			log.Printf(db_err.String())
			http.Error(w, "Server error", 501)
			return
		}
		defer config.database.Export(config.db_filename)

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
	dat, db_err := config.database.Get(key)
	if db_err != nil {
		log.Printf(db_err.String())
		http.Error(w, "Server error", 501)
		return
	}
	dat, _, err := decodeTime(dat)
	if err != nil {
		log.Printf(db_err.String())
		http.Error(w, "Server error", 501)
		return
	}
	p := &paste{Link: link, Contents: string(dat)}
	err = config.templates.ExecuteTemplate(w, "viewPage", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	err := config.templates.ExecuteTemplate(w, "errorPage", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

//startServer sets up the http server and routing to different end point handlers
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

//init_global sets up and initializes the global variables
func init_global(template_flag *string, static_flag *string, clean_db *bool) {
	template_dir := *template_flag
	if template_dir == "" {
		template_dir = os.Getenv("TEMPLATE_DIR")
	}
	config.templates = template.Must(template.ParseGlob(filepath.Join(template_dir, "*.html")))

	config.static_dir = *static_flag
	if config.static_dir == "" {
		config.static_dir = os.Getenv("STATIC_DIR")
	}

	var err error
	database_dir := filepath.Join(os.Getenv("HOME"), ".pastebin-go")
	config.db_filename = filepath.Join(database_dir, ".pastedata.kvdb")
	if _, err = os.Stat(database_dir); err != nil {
		if os.IsNotExist(err) {
			//Create
			os.Mkdir(database_dir, 0775)
		} else {
			panic(err)
		}
	}

	if _, err = os.Stat(config.db_filename); err == nil && *clean_db == true {
		os.Remove(config.db_filename)
	}

	config.database, err = kvdb.Open(config.db_filename, true)
	if err != nil {
		panic(err)
	}
}

func main() {
	//Setup command line flags
	var addrs listenAddresses
	flag.Var(&addrs, "interface", "Interface to listen to. Interfaces are of the form <address>:<port>. Call multiple times for multiple addresses")
	tmpl_flag := flag.String("template-dir", "", "Directory for template files")
	static_flag := flag.String("static-dir", "", "Directory for static files")
	clean_flag := flag.Bool("clean-database", false, "Delete existing database and create from scratch")
	flag.Parse()

	init_global(tmpl_flag, static_flag, clean_flag)
	defer config.database.Export(config.db_filename)

	if len(addrs) == 0 {
		addrs = append(addrs, "localhost:8080")
	}
	checkIfLowPort(addrs)

	var wg sync.WaitGroup
	for _, ifc := range addrs {
		wg.Add(1)
		go startServer(&wg, config.static_dir, ifc)
	}
	wg.Wait()
}
