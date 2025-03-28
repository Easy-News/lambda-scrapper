package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gocolly/colly/v2"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

func db_realated() {
	db, err := sql.Open(os.Getenv("DB"), os.Getenv("DATA_SOURCE")) // 1
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("DB 연동: %+v\n", db.Stats())

	result, err := db.Query("SHOW GLOBAL VARIABLES LIKE 'max_%resultect%'") // 1
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("resultection 생성: %+v\n", db.Stats()) // 2

	for result.Next() { // 3
		name := ""
		value := ""
		if err := result.Scan(&name, &value); err != nil {
			fmt.Println(err)
		}
		fmt.Println(name, value)
	}

	result, err = db.Query("SELECT * FROM instructor")
	if err != nil {
		log.Fatal(err)
	}

	for result.Next() {
		var id int
		var name string
		var dept_name string
		var salary int

		if err := result.Scan(&id, &name, &dept_name, &salary); err != nil {
			fmt.Println(err)
		}
		fmt.Println(id, name, dept_name, salary)
	}

	_, err = db.Exec(
		"INSERT INTO instructor (id, name, dept_name, salary) VALUES (7, 'gopher', 'CSE', 10000000)",
	)
	if err != nil {
		fmt.Println(err)
	}

	result.Close() // 4
	fmt.Printf("resultection 연결 종료: %+v\n", db.Stats())

	db.Close()
	fmt.Printf("DB 연동 종료: %+v\n", db.Stats())
}

func eachArticle(categoryString string, url string, ch chan<- result) {
	var title string
	var content string

	c := colly.NewCollector()

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36",
	}

	// Set a request callback to randomize user agent per request
	c.OnRequest(func(r *colly.Request) {
		// Choose a random user agent from the slice
		randomUA := userAgents[rand.Intn(len(userAgents))]
		r.Headers.Set("User-Agent", randomUA)
		fmt.Println("Visiting:", r.URL, "with User-Agent:", randomUA)
	})

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       2 * time.Second,
	})

	c.OnHTML("#title_area", func(e *colly.HTMLElement) {
		trimmedText := strings.TrimSpace(e.Text)
		cleanText := strings.Join(strings.Fields(trimmedText), " ")
		title = cleanText
	})

	c.OnHTML("article#dic_area", func(e *colly.HTMLElement) {
		trimmedText := strings.TrimSpace(e.Text)
		cleanText := strings.Join(strings.Fields(trimmedText), " ")
		content = cleanText
	})

	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Something went wrong:", err)
	})

	err := c.Visit(url)
	if err != nil {
		log.Fatal(err)
	}

	ch <- result{title, content, categoryString}
}

func collectLinks(category Category, ch chan<- urlWrapper) {
	var urls []string
	c := colly.NewCollector()

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36",
	}

	// Set a request callback to randomize user agent per request
	c.OnRequest(func(r *colly.Request) {
		// Choose a random user agent from the slice
		randomUA := userAgents[rand.Intn(len(userAgents))]
		r.Headers.Set("User-Agent", randomUA)
		fmt.Println("Visiting:", r.URL, "with User-Agent:", randomUA)
	})

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       2 * time.Second,
	})

	c.OnHTML("ul[id*='_SECTION_HEADLINE_LIST_'] .sa_text a[class*='sa_text_title']", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		urls = append(urls, link)
	})

	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Error:", err)
	})

	err := c.Visit(category.Url())
	if err != nil {
		log.Fatal(err)
	}

	ch <- urlWrapper{urls, category.String()}
}

type result struct {
	title    string
	category string
	content  string
}

type urlWrapper struct {
	urls     []string
	category string
}

func main() {
	newsCategory := []Category{
		Politic, Economy, Social, LivingCulture, ItScience, Global,
	}
	wrapperCh := make(chan urlWrapper)
	resultCh := make(chan result)
	for _, category := range newsCategory {
		go collectLinks(category, wrapperCh)
		urlWrappers := <-wrapperCh
		for _, url := range urlWrappers.urls {
			go eachArticle(urlWrappers.category, url, resultCh)
			data := <-resultCh
			fmt.Println(data)
		}
	}
}
