package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gocolly/colly/v2"
	"log"
	"os"
	"strings"
	"sync"
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

func collectLinks(category Category, ch chan<- urlWrapper, wg *sync.WaitGroup) {
	defer wg.Done()

	var urls []string
	c := colly.NewCollector()

	c.OnHTML("ul[id*='_SECTION_HEADLINE_LIST_'] .sa_text a[class*='sa_text_title']", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		urls = append(urls, link)
	})

	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Error in collectLinks:", err)
	})

	err := c.Visit(category.Url())
	if err != nil {
		log.Fatal(err)
	}

	ch <- urlWrapper{urls, category.String()}
}

// eachArticle scrapes an article for a given URL and sends the result through ch.
func eachArticle(categoryString string, url string, ch chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()

	var title string
	var content string

	c := colly.NewCollector()

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
		log.Println("Something went wrong in eachArticle:", err)
	})

	err := c.Visit(url)
	if err != nil {
		log.Fatal(err)
	}

	ch <- result{title, categoryString, content}
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
	var linksWg sync.WaitGroup
	wrapperCh := make(chan urlWrapper)

	var articlesWg sync.WaitGroup
	resultCh := make(chan result)

	for _, category := range newsCategory {
		linksWg.Add(1)
		go collectLinks(category, wrapperCh, &linksWg)
	}

	// Close the wrapperCh once all collectLinks goroutines are done.
	go func() {
		linksWg.Wait()
		close(wrapperCh)
	}()

	// Launch article scraping for each URL as they come in.
	// There’s no error because the for ... range syntax on a channel is a built-in Go feature that continuously receives values until the channel is closed.
	go func() {
		for wrapper := range wrapperCh {
			for _, url := range wrapper.urls {
				articlesWg.Add(1)
				go eachArticle(wrapper.category, url, resultCh, &articlesWg)
			}
		}
		// Once all article scraping goroutines are launched and done, close resultCh.
		articlesWg.Wait()
		close(resultCh)
	}()

	// Process the results concurrently.
	for res := range resultCh {
		fmt.Printf("Category: %s, Title: %s\nContent: %s\n\n", res.category, res.title, res.content)
	}
}
