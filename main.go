package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gophercises/quiet_hn/hn"
)

func main() {
	// parse flags
	var port, numStories int
	flag.IntVar(&port, "port", 3000, "the port to start the web server on")
	flag.IntVar(&numStories, "num_stories", 30, "the number of top stories to display")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

	http.HandleFunc("/", handler(numStories, tpl))

	// Start the server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handler(numStories int, tpl *template.Template) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		stories, err := getTopStories(numStories)
		// only one possible error, so this is ok
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		data := templateData{
			Stories: stories,
			Time:    time.Now().Sub(start),
		}
		err = tpl.Execute(w, data)
		if err != nil {
			http.Error(w, "Failed to process the template", http.StatusInternalServerError)
			return
		}
	})
}

// take in numStories - the number of stories a user wants; default is 30
// return a slice of items, which are the stories
func getTopStories(numStories int) ([]item, error) {
	// use the hn package to create a client and get the ids of the top ~450 items
	var client hn.Client
	ids, err := client.TopItems()
	if err != nil {
		return nil, errors.New("Failed to load top stories")
	}

	// create a result type and channel to pass results on
	type result struct {
		index int
		item  item
		err   error
	}
	resultCh := make(chan result)

	// create goroutines to get each hnItem,
	// creating a result and sending it to the channel
	for i := 0; i < numStories; i++ {
		go func(index, id int) {
			hnItem, err := client.GetItem(id)
			if err != nil {
				resultCh <- result{index: index, err: err}
			}
			resultCh <- result{index: index, item: parseHNItem(hnItem)}
		}(i, ids[i])
	}

	// create a results slice, appending each result as it is sent to the channel
	var results []result
	for i := 0; i < numStories; i++ {
		results = append(results, <-resultCh)
	}

	// sort the slice because the goroutines jumbled the order
	sort.Slice(results, func(i, j int) bool {
		return results[i].index < results[j].index
	})

	var stories []item

	for _, res := range results {
		// if the result has an error, skip it
		// continue goes to the next item in the for loop
		if res.err != nil {
			continue
		}
		if isStoryLink(res.item) {
			stories = append(stories, res.item)
		}
	}
	return stories, nil
}

func isStoryLink(item item) bool {
	return item.Type == "story" && item.URL != ""
}

func parseHNItem(hnItem hn.Item) item {
	ret := item{Item: hnItem}
	url, err := url.Parse(ret.URL)
	if err == nil {
		ret.Host = strings.TrimPrefix(url.Hostname(), "www.")
	}
	return ret
}

// item is the same as the hn.Item, but adds the Host field
type item struct {
	hn.Item
	Host string
}

type templateData struct {
	Stories []item
	Time    time.Duration
}
