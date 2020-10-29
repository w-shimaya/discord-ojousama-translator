package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/lib/pq"
)

// RegisteredWord : gorm table
type RegisteredWord struct {
	Id       int
	Source   string
	Target   string
	Relation sql.NullString
}

// Translate translates input sentence into Ojousama-Lang
func Translate(input string) string {

	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return fmt.Sprintln("error in initializing tokenizer", err)
	}

	// split into word list
	tokens := t.Analyze(input, tokenizer.Search)
	words := make([]string, 0, len(tokens))
	for _, v := range tokens {
		if v.Class != tokenizer.DUMMY && v.Surface != "" {
			words = append(words, v.Surface)
		}
	}

	// replace 'translatable' words
	databaseURL := os.Getenv("DATABASE_URL")
	db, err := gorm.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Sprintln("error in openning database,", err)
	}
	defer db.Close()

	ret := ""
	for _, w := range words {
		cand := []RegisteredWord{}
		//
		result := db.Find(&cand, "source=?", w)
		if result.Error != nil {
			return fmt.Sprintln("error in db query,", result.Error)
		}

		// translate
		if len(cand) > 0 {
			// [TODO] if the word has multiple candidates,
			//        choose one of them at random
			// [TODO] PoS based replacement
			//        e.g., prefix/suffix
			//        e.g., 終助詞 -> こと（ですね -> ですこと）
			//        e.g., か（終助詞） -> の（ますか -> ますの）
			// [TODO] not only replacing but also adding words
			// [TODO] use `relation` to determine translatability
			//        e.g., source | target | relation   | arg0 | ...
			//              です   | ですわ  | not before | わ   | ...
			// [TODO] consider better replacement logic
			//        such as maximizing `digree of fun'
			ret += cand[0].Target
		} else {
			// not registered word
			ret += w
		}
	}

	return ret
}
