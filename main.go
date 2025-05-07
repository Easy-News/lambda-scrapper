package main

import (
	"database/sql"
	"fmt"
	//_ "github.com/go-sql-driver/mysql"
	"github.com/gocolly/colly/v2"
	_ "github.com/lib/pq"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

var db *sql.DB

func formatWithQuotes(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

func initDB() {
	var err error
	db, err = sql.Open(os.Getenv("DB_MYSQL"), os.Getenv("DATA_SOURCE_MYSQL"))
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
	var images []string

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

	c.OnHTML("article#dic_area img", func(e *colly.HTMLElement) {
		// first try the normal src
		link := e.Attr("src")
		// if that’s empty, fall back to the lazy-load attribute
		if link == "" {
			link = e.Attr("data-src")
		}
		// make it absolute in case it’s relative
		link = e.Request.AbsoluteURL(link)
		images = append(images, link)
	})
	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Something went wrong in eachArticle:", err)
	})

	err := c.Visit(url)
	if err != nil {
		log.Fatal(err)
	}

	ch <- result{title, categoryString, content, images}
}

type result struct {
	title    string
	category string
	content  string
	images   []string
}

type urlWrapper struct {
	urls     []string
	category string
}

func main() {
	initDB()
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO news (title, content, category, images) VALUES ($1, $2, $3, $4)")
	//stmt, err := db.Prepare("INSERT INTO news (title, content, category, images) VALUES (?, ?, ?, ?)")
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
		_, err := stmt.Exec(res.title, res.content, res.category, formatWithQuotes(res.images))
		if err != nil {
			log.Println("Insert error:", err)
		} else {
			log.Printf("Record inserted: Title: %s, Category: %s\n\n", res.title, res.category)
		}
	}
}
