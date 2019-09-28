package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
)

type entry struct {
	URL   string
	Owner string
}

type page struct {
	Items   []entry
	Page    int
	HasNext bool
	Next    int
	HasPrev bool
	Prev    int
}

type server struct {
	pageSize int
	filename string
	data     []entry
}

// readEntries read entries from file
func (s *server) updateEntries() error {

	file, err := os.Open(s.filename)
	if err != nil {
		return err
	}

	defer func() {
		err = file.Close()
		if err != nil {
			log.Println("error while closing file")
		}
	}()

	var (
		out      []entry
		scanner  = bufio.NewScanner(file)
		badLines int
	)

	for scanner.Scan() {

		var row entry
		_, err = fmt.Sscanf(scanner.Text(), "%s %s", &row.URL, &row.Owner)
		if err != nil {
			badLines++
			continue
		}

		out = append(out, row)

	}

	// reverse the order
	for i := len(out)/2 - 1; i >= 0; i-- {
		opp := len(out) - 1 - i
		out[i], out[opp] = out[opp], out[i]
	}

	log.Printf("Updated picture data, loaded %v entries, skipped %v bad lines", len(out), badLines)

	s.data = out
	return nil
}

// select entries for this page only
func (s *server) paginate(page int) []entry {

	from := page * s.pageSize
	to := from + s.pageSize

	if to >= len(s.data) {
		to = len(s.data) - 1
	}

	if from < 0 || from >= len(s.data) || to <= from {
		return nil
	}

	return s.data[from:to]

}

// handle HTTP request
func (s *server) handleRequest(c *gin.Context) {

	pageID, _ := strconv.Atoi(c.DefaultQuery("page", "0"))

	items := s.paginate(pageID)

	if len(items) == 0 {
		c.Redirect(http.StatusTemporaryRedirect, "/404")
		return
	}

	context := page{
		Items:   items,
		Page:    pageID,
		HasNext: len(items) == s.pageSize,
		Next:    pageID + 1,
		HasPrev: pageID > 0,
		Prev:    pageID - 1,
	}

	c.HTML(http.StatusOK, "index.tmpl", context)
}

func createServer(perPage int, filename string) (*server, error) {

	s := &server{
		filename: filename,
		pageSize: perPage,
	}

	err := s.updateEntries()
	if err != nil {
		return nil, err
	}

	go s.watch()
	return s, nil
}

func (s *server) watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	err = watcher.Add(s.filename)
	if err != nil {
		log.Fatal(err)
	}

	// wait for events
	for range watcher.Events {

		err := s.updateEntries()
		if err != nil {
			log.Printf("error while realoading the file: %v", err)
			continue
		}
	}
}

func main() {

	filename := flag.String("filename", "", "Filename to read picture URLs from")
	perPage := flag.Int("per-page", 10, "Number of pictures to show per page")
	listenAddr := flag.String("listen-addr", "127.0.0.1:3032", "Address to listen on")
	templateDir := flag.String("template-dir", "templates/", "Directory to load templates from")

	flag.Parse()

	if *perPage == 0 || *filename == "" || *listenAddr == "" {
		flag.Usage()
		return
	}

	router := gin.Default()
	router.LoadHTMLGlob(filepath.Join(*templateDir, "*.tmpl"))

	server, err := createServer(*perPage, *filename)
	if err != nil {
		log.Fatalf("Error while creating the server: %v", err)
	}

	router.GET("/", server.handleRequest)

	log.Fatal(router.Run(":3032"))
}
