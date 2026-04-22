package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
)

var connStr = "user=postgres password=Runrq dbname=runrq_db sslmode=disable"

var db, err = sql.Open("postgres", connStr)

func ConnectDb() *sql.DB {
	if err != nil {
		panic(err)
	}

	return db
}

func CloseDb() {
	closingErr := db.Close()

	if closingErr != nil {
		fmt.Println("error occured in closing db connection", closingErr.Error())
	}
}