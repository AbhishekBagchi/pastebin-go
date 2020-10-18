package main

//FIXME Change all the random server errors to bettor messages

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"github.com/AbhishekBagchi/kvdb"
	"github.com/AbhishekBagchi/priorityqueue"
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

var timedEntryQueue *priorityqueue.PriorityQueue

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
	templates  *template.Template
	staticDir  string
	dbFilename string
	database   *kvdb.Database
}

var hash = sha256.New()

type paste struct {
	Link     template.URL
	Contents string
}

//encodeTime takes in the raw data being pasted and prepares it for storage in the database.
//The data is stored as <exprity_time uint64> + <paste_data>
//expiryTime is currentTime + TTL. TTL is in minutes
func encodeTime(data []byte, ttl time.Duration) (uint64, []byte) {
	//Pre-prend a canary for some robustness
	canary := []byte{0xde, 0xad}
	if ttl == 0 {
		timeBytes := make([]byte, 8)
		for i := range timeBytes {
			timeBytes[i] = 0xFF
		}
		prepend := append(canary, timeBytes...)
		data = append(prepend, data...)
		return 0xFFFFFFFFFFFFFFFF, data
	} else {
		currTime := time.Now()
		expiryTime := (uint64)(currTime.Add(ttl).Unix())
		timeBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(timeBytes, expiryTime)
		prepend := append(canary, timeBytes...)
		data = append(prepend, data...)
		return expiryTime, data
	}
}

//decodeTime gets the expiryTime time for a data chunk from the first 8 bytes
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
		ttlUnit := r.FormValue("ttl_unit")
		ttlFormValue := r.FormValue("ttl")
		var ttl time.Duration = 0
		if ttlFormValue != "" {
			ttlValue, err := strconv.Atoi(ttlFormValue)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			switch ttlUnit {
			case "minute":
				ttl = time.Duration(ttlValue) * time.Minute
			case "hour":
				ttl = time.Duration(ttlValue) * time.Hour
			case "day":
				ttl = time.Duration(ttlValue) * 24 * time.Hour
			}
		}
		expiryTime, data := encodeTime(data, ttl)
		hash := sha256.Sum256(data)
		key := hex.EncodeToString(hash[0:4])
		if expiryTime != 0xFFFFFFFFFFFFFFFF {
			timedEntryQueue.Push(key, int(expiryTime))
		}
		log.Printf("%s: %v", key, data)

		dbErr := config.database.Insert(key, data, false)
		if dbErr != nil {
			log.Printf(dbErr.String())
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
		defer config.database.Export(config.dbFilename)

		http.Redirect(w, r, "/view/"+key, http.StatusSeeOther)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	idx := strings.Index(urlPath, "/view/")
	if idx == -1 {
		errorHandler(w, r, http.StatusNotFound, "Not found")
		return
	}
	link := template.URL("http://" + r.Host + urlPath)
	key := urlPath[6:]
	dat, dbErr := config.database.Get(key)
	if dbErr != nil {
		errorHandler(w, r, http.StatusNotFound, dbErr.String())
		return
	}
	dat, _, err := decodeTime(dat)
	if err != nil {
		log.Printf(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("%v", dat)
	p := &paste{Link: link, Contents: string(dat)}
	err = config.templates.ExecuteTemplate(w, "viewPage", p)
	if err != nil {
		errorHandler(w, r, http.StatusNotFound, err.Error())
		return
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request, status int, msg string) {
	w.WriteHeader(status)
	err := config.templates.ExecuteTemplate(w, "errorPage", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

//startServer sets up the http server and routing to different end point handlers
func startServer(wg *sync.WaitGroup, staticDir string, ifc string) {
	defer wg.Done()
	log.Printf("Listening on %s", ifc)
	serveMux := http.NewServeMux()
	//Serve static CSS etc
	serveMux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	serveMux.HandleFunc("/", indexHandler)
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

//cleanupDB goes through all entries in the database and deletes any that should have expired
//FIXME this needs to also add entries back into the timedEntryQueue
func cleanupDB(db *kvdb.Database) {
	currTime := (uint64)(time.Now().Unix())
	for key, value := range db.ToRawMap() {
		_, expiryTime, err := decodeTime(value)
		if err != nil {
			panic(err)
		}
		if expiryTime < currTime {
			db.Delete(key)
			log.Printf("Removing key %s. Current Time %v. Expiry time %v", key, currTime, expiryTime)
			//The database is exported after every deletion
			config.database.Export(config.dbFilename)
		} else {
			//Push entries into the timed queue
			timedEntryQueue.Push(key, int(expiryTime))
		}
	}
}

func cleanupTimedQueue(ticker *time.Ticker, db *kvdb.Database) {
	for ; true; <-ticker.C {
		log.Printf("Ticker ran. Queue has %v elements", timedEntryQueue.Len())
		currTime := uint64(time.Now().Unix())
		if timedEntryQueue.Len() > 0 {
			value, expiryTime, err := timedEntryQueue.Top()
			if err != nil {
				panic(err)
			}
			key := value.(string)
			if uint64(expiryTime) < currTime {
				db.Delete(key)
				timedEntryQueue.Pop()
				log.Printf("Removing key %s. Current Time %v. Expiry time %v", key, currTime, expiryTime)
				//The database is exported after every deletion
				config.database.Export(config.dbFilename)
			}
		}
	}
}

//initGlobals sets up and initializes the global variables
func initGlobals(templateFlag *string, staticFlag *string, cleanDb *bool) {

	timedEntryQueue = priorityqueue.New(priorityqueue.MinQueue)

	templateDir := *templateFlag
	if templateDir == "" {
		templateDir = os.Getenv("TEMPLATE_DIR")
	}
	config.templates = template.Must(template.ParseGlob(filepath.Join(templateDir, "*.html")))

	config.staticDir = *staticFlag
	if config.staticDir == "" {
		config.staticDir = os.Getenv("STATIC_DIR")
	}

	var err error
	databaseDir := filepath.Join(os.Getenv("HOME"), ".pastebin-go")
	config.dbFilename = filepath.Join(databaseDir, ".pastedata.kvdb")
	if _, err = os.Stat(databaseDir); err != nil {
		if os.IsNotExist(err) {
			//Create
			os.Mkdir(databaseDir, 0775)
		} else {
			panic(err)
		}
	}

	if _, err = os.Stat(config.dbFilename); err == nil && *cleanDb == true {
		os.Remove(config.dbFilename)
	}

	config.database, err = kvdb.Open(config.dbFilename, true)
	cleanupDB(config.database)
	if err != nil {
		panic(err)
	}
}

func main() {
	//Setup command line flags
	var addrs listenAddresses
	flag.Var(&addrs, "interface", "Interface to listen to. Interfaces are of the form <address>:<port>. Call multiple times for multiple addresses")
	tmplFlag := flag.String("template-dir", "", "Directory for template files")
	staticFlag := flag.String("static-dir", "", "Directory for static files")
	cleanFlag := flag.Bool("clean-database", false, "Delete existing database and create from scratch")
	flag.Parse()

	initGlobals(tmplFlag, staticFlag, cleanFlag)
	defer config.database.Export(config.dbFilename)

	cleanupQueueTicker := time.NewTicker(30 * time.Second)
	defer cleanupQueueTicker.Stop()

	go cleanupTimedQueue(cleanupQueueTicker, config.database)

	if len(addrs) == 0 {
		addrs = append(addrs, "localhost:8080")
	}
	checkIfLowPort(addrs)

	var wg sync.WaitGroup
	for _, ifc := range addrs {
		wg.Add(1)
		go startServer(&wg, config.staticDir, ifc)
	}
	wg.Wait()
}
