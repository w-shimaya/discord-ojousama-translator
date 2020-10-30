package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/lib/pq"
)

// RegisteredWord : gorm table
type RegisteredWord struct {
	Id            int
	SourceSurface string
	SourcePos     string
	TargetSurface string
}

// Translate translates input sentence into Ojousama-Lang
func Translate(input string) string {

	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return fmt.Sprintln("error in initializing tokenizer", err)
	}

	// split into word list
	tokens := t.Analyze(input, tokenizer.Search)

	// replace 'translatable' words
	databaseURL := os.Getenv("DATABASE_URL")
	db, err := gorm.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Sprintln("error in openning database,", err)
	}
	defer db.Close()

	ret := ""
	precedingPos := ""
	for i, token := range tokens {
		if token.Class == tokenizer.DUMMY || token.Surface == "" {
			continue
		}

		// prefix addition
		// ! these process should be refactored
		//   to have more generality
		// 連続する名詞の頭に「お」
		pos := token.POS()
		if pos[0] == "名詞" &&
			(pos[1] == "一般" || pos[1] == "サ変接続" || pos[1] == "数") {
			// 先頭にあるか，一つ前が名詞でない
			if i == 0 || precedingPos != "名詞" {
				ret += "お"
			}
		}
		precedingPos = pos[0]

		// suffix addition
		// explicit EOS
		if token.Surface == "。" && i-1 > 0 &&
			tokens[i-1].POS()[0] != "助詞" &&
			tokens[i-1].POS()[0] != "記号" {
			// e.g., ました。 -> ましたわ。
			//       した    -> したの。
			// at random (at 50% probability)
			rand.Seed(time.Now().UnixNano())
			p := rand.Float32()
			if p < 0.5 {
				ret += "わ"
			} else {
				ret += "の"
			}
		}
		// implicit EOS
		if i == len(tokens)-1 &&
			(token.POS()[0] != "助詞" &&
				token.POS()[0] != "記号" &&
				token.POS()[0] != "名詞") {

			rand.Seed(time.Now().UnixNano())
			p := rand.Float32()
			if p < 0.5 {
				ret += token.Surface + "わ"
			} else {
				ret += token.Surface + "の"
			}
			break
		}

		// look up database
		cand := []RegisteredWord{}
		result := db.Find(&cand, "source_surface=?", token.Surface)
		if result.Error != nil {
			return fmt.Sprintln("error in db query,", result.Error)
		}

		// translate
		if len(cand) > 0 {
			// [TODO] if the word has multiple candidates,
			//        choose one of them at random
			// [TODO] PoS based replacement
			//        e.g., 終助詞 -> こと（ですね -> ですこと）
			//        e.g., か（終助詞） -> の（ますか -> ますの）
			// [TODO] not only replacing but also adding words
			// [TODO] use `relation` to determine translatability
			//        e.g., source | target | relation   | arg0 | ...
			//              です   | ですわ  | not before | わ   | ...
			// [TODO] consider better replacement logic
			//        such as maximizing `digree of fun'
			addstr := token.Surface
			for _, c := range cand {
				// replace if the PoS matches
				posStr := strings.Join(token.POS(), ",")
				if strings.HasPrefix(posStr, c.SourcePos) {
					addstr = c.TargetSurface
				}
			}
			ret += addstr
		} else {
			// not registered word
			ret += token.Surface
		}
	}

	return ret
}
