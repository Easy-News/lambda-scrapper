package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"log"
)

func main() {
	db, err := sql.Open("mysql", "root:9036@tcp(127.0.0.1:3306)/example-db") // 1
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("DB 연동: %+v\n", db.Stats())    // 2
	db.Close()                                // 3
	fmt.Printf("DB 연동 종료: %+v\n", db.Stats()) // 4
}
