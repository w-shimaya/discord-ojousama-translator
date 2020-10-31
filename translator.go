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
			(pos[1] == "一般" || pos[1] == "サ変接続" || pos[1] == "数" || pos[1] == "形容動詞語幹") {
			// 先頭にあるか，一つ前が名詞，接頭詞でない
			if i == 0 || (precedingPos != "名詞" && precedingPos != "接頭詞") {
				ret += "お"
			}
		}
		precedingPos = pos[0]

		// look up database
		cand := []RegisteredWord{}
		posStr := strings.Join(token.POS(), ",")
		result := db.Where("(source_surface=? OR source_surface IS NULL) AND (? LIKE source_pos || '%' OR source_pos IS NULL)", token.Surface, posStr).Find(&cand)
		if result.Error != nil {
			return fmt.Sprintln("error in db query,", result.Error)
		}

		// translate
		if len(cand) > 0 {
			// [TODO] consider better replacement logic
			//        such as maximizing `digree of fun'

			// if the word has multiple candidates, choose one of them at random
			rand.Seed(time.Now().UnixNano())
			p := rand.Intn(len(cand))
			ret += cand[p].TargetSurface
		} else {
			// not registered word
			ret += token.Surface
		}

		// suffix addition
		// 丁寧語変換
		if token.POS()[0] == "動詞" {
			// collect required info about the next token
			var nextPos string
			var nextBase string
			var nextSurface string
			if i+1 < len(tokens) {
				nextPos = tokens[i+1].POS()[0]
				nextBase, _ = tokens[i+1].BaseForm()
				nextSurface = tokens[i+1].Surface
			}

			// 動詞で終わる or 動詞のすぐ後に「ます」「て」以外の助詞助動詞あるいは句点が続く
			// => 丁寧語でないとみなす
			if i == len(tokens)-1 || nextPos == "句点" ||
				(nextPos == "助詞" && nextBase != "て") ||
				(nextPos == "助動詞" && nextBase != "ます") {
				// 動詞を連用形に活用する
				conj := ConjugateVerb(token, renyo)
				// remove overlapping
				runeconj := []rune(conj)
				runeret := []rune(ret)
				for i := len(runeret) - 1; i >= 0; i-- {
					if runeconj[0] == runeret[i] {
						ret = string(runeret[:i])
						break
					}
				}
				// concat conjugated verb
				ret += conj
				// 「ます」を適切な活用の上追加する
				ret += Conjugate("ます", nextBase, nextPos)
				// [TBC] しない -> しません
				if nextPos == "助動詞" && nextSurface == "ない" {
					tokens[i+1].Surface = "ん"
				}
			}
		}

		// explicit EOS
		if token.POS()[0] == "句点" && i > 0 &&
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
				ret += "わ"
			} else {
				ret += "の"
			}
		}
	}

	return ret
}
