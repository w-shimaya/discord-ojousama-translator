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
	var precedingPos []string
	for i, token := range tokens {
		if token.Class == tokenizer.DUMMY || token.Surface == "" {
			continue
		}

		// prefix addition
		// ! these process should be refactored
		//   to have more generality
		// 連続する名詞の頭に「お」
		pos := token.POS()
		if pos[0] == "名詞" && (pos[1] == "一般" || pos[1] == "サ変接続" || pos[1] == "数" || pos[1] == "形容動詞語幹") {
			// 先頭にあるか，一つ前が名詞，接頭詞，でない．ただし副詞可能名詞ならよい
			if i == 0 || (precedingPos[0] != "名詞" && precedingPos[0] != "接頭詞") || precedingPos[1] == "副詞可能" {
				ret += "お"
			}
		}
		precedingPos = pos

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
				// 命令形
				form, _ := token.FeatureAt(5)
				if strings.HasPrefix(form, "命令") {
					conj := ConjugateVerb(token, renyoTe)
					// remove overlapping
					runeret := []rune(ret)
					surflen := len([]rune(token.Surface))
					ret = string(runeret[:len(runeret)-surflen])
					// concat verb
					ret += conj
					// add 「てくださいませ」
					ret += "てくださいませ"
				} else {
					// 動詞を連用形に活用する
					conj := ConjugateVerb(token, renyo)
					// remove overlapping
					runeret := []rune(ret)
					surflen := len([]rune(token.Surface))
					ret = string(runeret[:len(runeret)-surflen])
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
		}

		// EOS translation
		// 1. 「わ」「の」 (normal sentence)
		// 2. 「かしら」 (interrogative sentence)
		// [TBC] these process will be improved to utilize database
		// [TODO:refactor] much portion of explicit & implicit EOS procedure looks very similar

		// explicit EOS
		if token.POS()[0] == "記号" && i > 0 && tokens[i-1].POS()[0] != "助詞" && tokens[i-1].POS()[0] != "記号" && tokens[i-1].POS()[0] != "感動詞" && tokens[i-1].Surface != "う" {
			retrune := []rune(ret)
			preForm, _ := tokens[i-1].FeatureAt(5)
			if strings.HasPrefix(preForm, "命令") {
				// imperative sentence
				// no suffix
			} else if token.Surface == "？" {
				// interrogate sentence
				if tokens[i-1].Surface == "か" {
					ret = string(retrune[:len(retrune)-2]) + "かしら" + token.Surface
				} else {
					ret = string(retrune[:len(retrune)-1]) + "かしら" + token.Surface
				}
			} else {
				// normal sentence
				desu := ""
				if tokens[i-1].POS()[0] == "名詞" {
					desu = "です"
				}

				// at random (at 50% probability)
				rand.Seed(time.Now().UnixNano())
				p := rand.Float32()
				if p < 0.5 {
					ret = string(retrune[:len(retrune)-1]) + desu + "わ" + token.Surface
				} else {
					ret = string(retrune[:len(retrune)-1]) + desu + "の" + token.Surface
				}
			}
		}

		// implicit EOS
		if i == len(tokens)-1 &&
			(token.POS()[0] != "助詞" &&
				token.POS()[0] != "記号" &&
				token.POS()[0] != "名詞" &&
				token.POS()[0] != "感動詞" &&
				token.Surface != "う") {

			preForm, _ := token.FeatureAt(5)
			if strings.HasPrefix(preForm, "命令") {
				// imperative sentence
				// no suffix
			} else if token.Surface == "か" {
				// interrogative sentence
				retrune := []rune(ret)
				ret = string(retrune[:len(retrune)-1]) + "かしら"
			} else {
				// normal sentence
				rand.Seed(time.Now().UnixNano())
				p := rand.Float32()

				desu := ""
				base, _ := token.BaseForm()
				if tokens[i-1].POS()[0] == "名詞" && base != "だ" && base != "です" {
					desu = "です"
				}

				// at random (at 50% probability)
				if p < 0.5 {
					ret += desu + "わ"
				} else {
					ret += desu + "の"
				}
			}
		}
	}

	return ret
}
