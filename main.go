package main

import (
	"context"
	"database/sql"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gocolly/colly/v2"
	_ "github.com/lib/pq"
	"log"
	"os"
	"strings"
	"sync"
	"time"
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
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
			"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 Safari/537.36"),
	)

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
	})

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

func Handler(ctx context.Context) (string, error) {
	initDB()
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO news (title, content, category) VALUES ($1, $2, $3)")
	if err != nil {
		log.Fatal(err)
		return "", err // Lambda의 경우 오류가 났다고 알려줘야 해
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
			log.Fatal("Insert error:", err)
		} else {
			log.Printf("Record inserted: Title: %s, Category: %s\n\n", res.title, res.category)
		}
	}

	return "Scraping and insertion completed", nil
}

func main() {
	lambda.Start(Handler)
}
