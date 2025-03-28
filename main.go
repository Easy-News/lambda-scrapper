package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gocolly/colly/v2"
	"log"
	"os"
	"strings"
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

func eachArticle(categoryString string, url string) (results result) {
	c := colly.NewCollector()

	c.OnHTML("#title_area", func(e *colly.HTMLElement) {
		trimmedText := strings.TrimSpace(e.Text)
		cleanText := strings.Join(strings.Fields(trimmedText), " ")
		results.title = cleanText
	})

	c.OnHTML("article#dic_area", func(e *colly.HTMLElement) {
		trimmedText := strings.TrimSpace(e.Text)
		cleanText := strings.Join(strings.Fields(trimmedText), " ")
		results.content = cleanText
	})

	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Something went wrong:", err)
	})

	err := c.Visit(url)
	if err != nil {
		log.Fatal(err)
	}

	results.category = categoryString
	return
}

func collectLinks(category Category) (categoryString string, url []string) {
	c := colly.NewCollector()

	c.OnHTML("ul[id*='_SECTION_HEADLINE_LIST_'] .sa_text a[class*='sa_text_title']", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		url = append(url, link)
	})

	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Error:", err)
	})

	err := c.Visit(category.Url())
	if err != nil {
		log.Fatal(err)
	}

	categoryString = category.String()
	return
}

type result struct {
	title    string
	category string
	content  string
}

func main() {
	newsCategory := []Category{
		Politic, Economy, Social, LivingCulture, ItScience, Global,
	}

	for _, category := range newsCategory {
		categoryString, urls := collectLinks(category)
		for _, url := range urls {
			data := eachArticle(categoryString, url)
			fmt.Println(data)
		}
	}
}
