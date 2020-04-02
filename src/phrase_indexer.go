package main

import (
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"sort"
	"log"
)

type phraseCnt struct {
	phrase string
	count  uint32
}

func main() {
	threadURL, selector, _, start, end, nworkers, limit := getArguments()

	jobSize := end - start + 1
	if nworkers > jobSize {
		nworkers = jobSize
	}

	collectorChan := make(chan *[]phraseCnt)
	result := make(chan *map[string]uint32)

	go collector(collectorChan, result)

	jobs, wg := spawnWorkers(selector, nworkers, collectorChan)
	initJobs(jobs, threadURL, start, end)



	wg.Wait()
	close(collectorChan)

	printOutRanking( <-result , limit )

}

func printOutRanking(phraseCounts *map[string]uint32, limit int) {
	ranking := sortByPhraseCount(phraseCounts)

	for i, elem := range *ranking {
		if i >= limit {
			break
		}

		fmt.Printf("%v \t\t\t% v\n", elem.count, elem.phrase)
	}
}

func collector(input <-chan *[]phraseCnt, result chan<- *map[string]uint32) {
	state := make(map[string]uint32)

	for iteration := range input {
		for _, tuple := range *iteration {
			state[tuple.phrase] += tuple.count
		}
	}

	result <- &state
}

func spawnWorkers(selector string, howMany uint, collector chan<- *[]phraseCnt) (
	jobs chan string, wg *sync.WaitGroup) {
	jobs = make(chan string)

	var _wg sync.WaitGroup
	wg = &_wg

	var i uint
	for i = 0; i < howMany; i++ {
		wg.Add(1)
		go worker(
			selector,
			jobs,
			collector,
			wg)
	}

	return
}

func initJobs(jobs chan<- string, threadURL string, start uint, end uint) {
	for i := start; i <= end; i++ {
		jobs <- threadURL + fmt.Sprintf("%v", i)
	}
	close(jobs)
}

func worker(selector string, jobs <-chan string, collector chan<- *[]phraseCnt, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		resp := getResponse(job)
		doc := *getHtml(resp)
		defer resp.Body.Close()

		phrasesCount := make(map[string]uint32)

		doc.Find(selector).Each(func(i int, selection *goquery.Selection){

			rawText := selection.Text()
			normalize(&rawText)

			phrases := strings.Fields(rawText)

			for _, phrase := range phrases {
				phrasesCount[phrase]++
			}

		})

		var phrasesCountAsSlice []phraseCnt

		for phrase, cnt := range phrasesCount {
			phrasesCountAsSlice = append(phrasesCountAsSlice, phraseCnt{ phrase, cnt })
		}

		collector <- &phrasesCountAsSlice
		log.Printf("Job done: %v\n", job)
	}
}

func getResponse(site string) *http.Response {
	resp, err := http.Get(site)

	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		panic(fmt.Sprintf("Got response status code equal %d. Aborting.",resp.StatusCode))
	}

	return resp
}

func getHtml(resp *http.Response) *goquery.Document {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		panic(err)
	}

	return doc
}

func getArguments() (threadURL, selector, exclude string, start, end, workers uint, limit int) {
	const sREQUIRED = ""
	const iREQUIRED = 0
	const DEFAULT_WORKERS = 100

	flag.StringVar(&threadURL, "threadURL", sREQUIRED,
		"[REQUIRED] URL to threadURL that is meant to be indexed")
	flag.StringVar(&selector, "selector", sREQUIRED,
		"[REQUIRED] Selector for searching for interesting parts of the document")
	flag.UintVar(&start, "start", 1,
		"[OPTIONAL] Page number on which to start indexing")
	flag.UintVar(&end, "end", iREQUIRED,
		"[REQUIRED] Page number on which to end indexing")

	flag.StringVar(&exclude, "exclude", "",
		"[OPTIONAL] Path to file that contains phrases to exclude from output")
	flag.IntVar(&limit, "limit", math.MaxInt32,
		"[OPTIONAL] Limit output to top #{value} entries")
	flag.UintVar(&workers, "workers", DEFAULT_WORKERS,
		"[OPTIONAL] Number of workers involved to parsing thread sites")

	flag.Parse()

	if end == 0 || threadURL == "" || selector == ""  {
		fmt.Fprintf(os.Stderr, "Missing arguments; --help for more information\n")
		os.Exit(1)
	}

	return
}

func normalize (text *string) {
	nonWord, _ := regexp.Compile("[0-9`~!@#$%^&*()_+-=\\[\\]{}|'\";:/.,><]")
	*text = nonWord.ReplaceAllLiteralString(*text, "")
	*text = strings.ToLower(*text)
	replacePolishDiacritics(text)
}

func replacePolishDiacritics(text *string) {
	a, _ := regexp.Compile("ą")
	c, _ := regexp.Compile("ć")
	e, _ := regexp.Compile("ę")
	l, _ := regexp.Compile("ł")
	o, _ := regexp.Compile("ó")
	s, _ := regexp.Compile("ś")
	z, _ := regexp.Compile("[żź]")

	replacements := map[*regexp.Regexp]string{
		a : "a",
		c : "c",
		e : "e",
		l : "l",
		o : "o",
		s : "s",
		z : "z",
	}

	for regexptr, repl := range replacements {
		*text = regexptr.ReplaceAllLiteralString(*text, repl)
	}
}

func sortByPhraseCount(phraseCounts *map[string]uint32) *[]phraseCnt {
	ranking := make([]phraseCnt, len(*phraseCounts))

	var i uint32 = 0
	for phrase, cnt := range *phraseCounts {
		ranking[i].phrase = phrase
		ranking[i].count = cnt

		i++
	}

	sort.Slice(ranking, func(i, j int) bool {
		return ranking[i].count > ranking[j].count
	})

	return &ranking
}
