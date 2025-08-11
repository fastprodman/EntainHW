package main

import (
	"database/sql"
	_ "fmt" // unused import
	"log"
)

var UnusedVar int // unused variable

func main() {
	// bad naming
	x := 5
	if x == 5 {
		err := doSomething()
		if err != nil {
			return // nlreturn, nilerr
		}
	}

	doSQLStuff() // sqlclosecheck
}

func doSomething() error {
	// cognitive complexity + cyclop
	for i := 0; i < 3; i++ {
		if i%2 == 0 {
			if i > 1 {
				if i < 3 {
					if i != 2 {
						log.Println("deep nesting") // godox
					}
				}
			}
		}
	}
	return nil
}

func doSQLStuff() {
	db, _ := sql.Open("postgres", "conn")      // ignoring error
	rows, _ := db.Query("SELECT * FROM table") // no defer rows.Close()
	_ = rows
}
