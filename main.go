package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gocolly/colly/v2"
	//_ "github.com/lib/pq"
	openai "github.com/sashabaranov/go-openai"
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

	ch <- result{title, categoryString, "", url, content, images}
}

func computeSubCategory(request string) (respContent string) {
	client := openai.NewClient(os.Getenv("GPT"))
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(`다음 기사 제목들에 대해 카테고리를 분류해줘. 카테고리는 다음과 같이 있어. 나의 국적은 대한민국이야, 국내, 해외를 판별할때 사용해
DOMESTIC_POLITICS, ELECTION_AND_PRESIDENTIAL, INTERNATIONAL_POLITICS_AND_DIPLOMACY, ECONOMIC_POLICY, CORPORATE_AND_INDUSTRY_TRENDS, FINANCE_AND_SECURITIES, IT_AND_SCIENCE_TECHNOLOGY, TELECOMMUNICATION_AND_MOBILE, SOCIETY_AND_WELFARE, INCIDENT_AND_ACCIDENT, LEGAL_AND_SECURITY, ENVIRONMENT_AND_CLIMATE, CULTURE_AND_ART, ENTERTAINMENT_AND_BROADCASTING, SPORTS, HEALTH_AND_MEDICAL, EDUCATION_AND_ADMISSIONS, REAL_ESTATE_AND_CONSTRUCTION, TRAVEL_AND_LEISURE, COLUMN_AND_OPINION
다음은 너가 분류해야할 기사 제목이야:
%s
다음 예시 형식과 같이 출력해줘, 카테고리만 출력해주면 돼, 내가 보내준 기사 순서와 너가 분류한 카테고리의 출력 순서는 같아야해, 각 기사제목은 ", 제목끝\n" 으로 구분 되어 있어.
가사 제목의 갯수와 너가 출력할 카테고리의 갯수는 같아야해, 만약 같지 않다면 같아질때 까지 반복해줘
예시) SPORTS, HEALTH_AND_MEDICAL, CULTURE_AND_ART`, request),
				},
			},
		},
	)
	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}
	respContent = resp.Choices[0].Message.Content
	return
}

type result struct {
	title        string
	category     string
	sub_category string
	article_url  string
	content      string
	images       []string
}

type urlWrapper struct {
	urls     []string
	category string
}

func main() {
	initDB()
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO news (title, content, category, sub_category, image_url, article_url, news_type) VALUES (?, ?, ?, ?, ?, ?, 'HEADLINE')")
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

	var results []result
	var titles []string
	for res := range resultCh {
		results = append(results, res)
		titles = append(titles, res.title)

	}
	// Batch GPT requests in groups of 10 to avoid large payloads
	var allSubCats []string
	batchSize := 10
	for start := 0; start < len(titles); start += batchSize {
		end := start + batchSize
		if end > len(titles) {
			end = len(titles)
		}
		batchTitles := titles[start:end]
		joined := strings.Join(batchTitles, ", 제목끝\n")
		resp := computeSubCategory(joined)
		resp = strings.TrimSpace(resp)
		cats := strings.Split(resp, ", ")
		if len(cats) != len(batchTitles) {
			log.Printf("❗️ Batch 분류 개수(%d)와 배치 기사 개수(%d)가 다릅니다. 부족한 부분은 UNCATEGORIZED로 채웁니다.", len(cats), len(batchTitles))
			for len(cats) < len(batchTitles) {
				cats = append(cats, "UNCATEGORIZED")
			}
		}
		allSubCats = append(allSubCats, cats...)
	}

	// Assign sub_category and insert each record
	for i := range results {
		results[i].sub_category = allSubCats[i]
		_, err := stmt.Exec(
			results[i].title,
			results[i].content,
			results[i].category,
			results[i].sub_category,
			formatWithQuotes(results[i].images),
			results[i].article_url,
		)
		if err != nil {
			log.Println("Insert error:", err)
		} else {
			log.Printf("Record inserted: Title: %s, Category: %s\n\n", results[i].title, results[i].category)
		}
	}
}
