package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
	_ "github.com/lib/pq"
)

// read environment variables related to database
// dbname, username, password
func getDatabaseConfig() (string, string, string) {

	dbname := os.Getenv("PGDBNAME")
	dbuser := os.Getenv("PGUSER")
	dbpass := os.Getenv("PGPASS")
	return dbname, dbuser, dbpass
}

// Translate translates input sentence into Ojousama-Lang
func Translate(input string) string {

	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		fmt.Println("error in initializing tokenizer", err)
		return "Internal Error"
	}

	// split into word list
	tokens := t.Analyze(input, tokenizer.Search)
	words := make([]string, 0, len(tokens))
	for _, v := range tokens {
		if v.Class != tokenizer.DUMMY && v.Surface != "" {
			words = append(words, v.Surface)
		}
	}

	// [TODO] replace 'translatable' words
	dbname, user, pass := getDatabaseConfig()
	db, err := sql.Open(
		"postgres",
		fmt.Sprintf("user=%s dbname=%s password=%s sslmode=disable", user, dbname, pass),
	)

	if err != nil {
		fmt.Println("error in openning database,", err)
		return "Internal Error"
	}
	defer db.Close()

	ret := ""
	for _, w := range words {
		rows, err := db.Query("SELECT target FROM ojousamaDict WHERE source='" + w + "';")
		if err != nil {
			fmt.Println("error in db query,", err)
			fmt.Println("word: ", w)
			return "Internal Error"
		}
		defer rows.Close()

		// translate
		cand := make([]string, 0)
		for rows.Next() {
			var target string
			if err := rows.Scan(&target); err != nil {
				fmt.Println("error in scanning rows,", err)
				return "Internal Error"
			}
			cand = append(cand, target)
		}

		rerr := rows.Close()
		if rerr != nil {
			fmt.Println("error in closing rows,", rerr)
			return "Internal Error"
		}

		if len(cand) > 0 {
			// [TODO] if the word has multiple candidates,
			// choose one of them at random
			ret += cand[0]
		} else {
			// not registered word
			ret += w
		}
	}

	return ret
}
