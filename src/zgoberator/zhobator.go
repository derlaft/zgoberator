package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/howeyc/fsnotify"
)

const pageSize = 10
const file = "links.txt"

type entry struct {
	URL   string
	Owner string
}

type page struct {
	Items []entry
	Pages []int
	Page  int
}

type server struct {
	data  []entry
	pages []int
}

// readEntries read entries from file
func readEntries() (out []entry) {
	file, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		tok := strings.Split(scanner.Text(), " ")

		if len(tok) == 2 {
			out = append(out, entry{tok[0], tok[1]})
		}

	}
	return
}

// update paginator info (which is a list of page numbers %)
func (s *server) updatePages() {

	s.pages = make([]int, 0, len(s.data)/pageSize)

	for i := 0; i*pageSize <= len(s.data); i++ {
		s.pages = append(s.pages, i)
	}
}

// select entries for this page only
func paginate(data []entry, page int) []entry {

	from := page * pageSize
	to := from + pageSize

	out := make([]entry, 0, pageSize)

	// use dumb loop to avoid useless and obscure ifs
	for i := range data {
		if i >= from && i < to {
			revIndex := len(data) - i - 1 // revI is "reversed" index; we want count real elements from the end
			out = append(out, data[revIndex])
		}
	}

	return out

}

// handle HTTP request
func (s *server) handleRequest(c *gin.Context) {

	pageID, _ := strconv.Atoi(c.DefaultQuery("page", "0"))

	items := paginate(s.data, pageID)

	if len(items) == 0 {
		c.Redirect(http.StatusTemporaryRedirect, "/404")
		return
	}

	context := page{
		Items: items,
		Page:  pageID,
		Pages: s.pages,
	}

	c.HTML(http.StatusOK, "index.tmpl", context)
}

func createServer() *server {
	s := &server{
		data: readEntries(),
	}

	s.updatePages()

	go s.watch()
	return s
}

func (s *server) watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	err = watcher.Watch(file)
	if err != nil {
		log.Fatal(err)
	}

	// wait for events
	for {
		select {
		case <-watcher.Event:
			s.data = readEntries()
			s.updatePages()
			log.Println(s.data, s.pages)
		}
	}
}

func main() {
	router := gin.Default()
	router.LoadHTMLGlob("*.tmpl")

	server := createServer()
	router.GET("/", server.handleRequest)

	router.Run(":3032")
}
