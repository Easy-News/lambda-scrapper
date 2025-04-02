package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gocolly/colly/v2"
	"time"
	//_ "github.com/lib/pq"
	"log"
	"os"
	"strings"
	"sync"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open(os.Getenv("DB"), os.Getenv("DATA_SOURCE"))
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	log.Println("Database connection initialized.")
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
	initDB()
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO news (title, content, category) VALUES (?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

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

	go func() {
		linksWg.Wait()
		close(wrapperCh)
	}()

	go func() {
		for wrapper := range wrapperCh {
			for _, url := range wrapper.urls {
				articlesWg.Add(1)
				go eachArticle(wrapper.category, url, resultCh, &articlesWg)
			}
		}
		articlesWg.Wait()
		close(resultCh)
	}()

	for res := range resultCh {
		_, err := stmt.Exec(res.title, res.content, res.category)
		if err != nil {
			log.Println("Insert error:", err)
		} else {
			log.Printf("Record inserted: Title: %s, Category: %s\n\n", res.title, res.category)
		}
	}
}
