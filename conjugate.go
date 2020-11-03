package main

import (
	"strings"

	"github.com/ikawaha/kagome/v2/tokenizer"
)

// ConjugType type
type ConjugType int

const (
	mizen ConjugType = iota
	renyo
	renyoTe
	shushi
	rentai
	katei
	meirei
)

// basic conjugation according to aux
func auxConjug(aux string) ConjugType {
	// [TODO] exceptions 「そう/だ」「まい」「よう」・「ようだ」
	switch aux {
	case "れる", "られる", "せる", "ない", "ぬ", "ん", "う":
		return mizen
	case "たい", "た", "だ", "ます", "やがる":
		return renyo
	case "らしい", "べき":
		return shushi
	default:
		return shushi // TBC
	}
}

// Conjugate `v` according to `base` `pos`
func Conjugate(v string, base string, pos string) string {
	// [TODO] maybe needs 'conjugation database'
	switch v {
	case "ます":
		t := auxConjug(base)
		// exceptions
		if base == "ん" || base == "ぬ" || base == "ない" {
			t = katei // not true but for convenience
		}
		if base == "まい" {
			t = shushi
		}
		switch t {
		case mizen:
			return "ましょ"
		case renyo, renyoTe:
			return "まし"
		case shushi, rentai:
			return "ます"
		case katei:
			return "ませ"
		case meirei:
			return "ませ"
		}

	}
	return ""
}

// ConjugateVerb for verbs
func ConjugateVerb(token tokenizer.Token, conjug ConjugType) string {
	// verb must have features
	infType, _ := token.FeatureAt(4)
	b, _ := token.BaseForm()

	switch {
	case strings.HasPrefix(infType, "五段"):
		// 五段・〇行～
		// 0 1 2 3
		column := []rune(infType)[3]
		// get stem
		stem := []rune(b)
		stem = stem[:len(stem)-1]
		// get row
		var row rune
		switch conjug {
		case mizen:
			row = 0
		case renyo:
			row = 1
		case shushi, rentai:
			row = 2
		case katei:
			row = 3
		case meirei:
			row = 4
		case renyoTe:
			// euphonic change
			switch column {
			case 'カ', 'ガ':
				return string(stem) + "い"
			case 'ア', 'タ', 'ラ', 'ワ':
				return string(stem) + "っ"
			case 'ナ', 'バ', 'マ':
				return string(stem) + "ん"
			default:
				row = 1
			}
		}
		retstr := string(stem) + getCharFromColumnRow(column, row)
		return retstr
	case strings.HasPrefix(infType, "一段"):
		// get stem
		stem := []rune(b)
		stem = stem[:len(stem)-1]
		// conjug
		switch conjug {
		case mizen, renyo, renyoTe:
			return string(stem)
		case shushi, rentai:
			return string(stem) + "る"
		case katei:
			return string(stem) + "れ"
		case meirei:
			return string(stem) + "ろ"
		}
	case strings.HasPrefix(infType, "カ変"):
		if []rune(token.Surface)[0] == '来' {
			switch conjug {
			case mizen, renyo, renyoTe:
				return "来"
			case shushi, rentai:
				return "来る"
			case katei:
				return "来れ"
			case meirei:
				return "来い"
			}
		} else {
			switch conjug {
			case mizen:
				return "こ"
			case renyo, renyoTe:
				return "き"
			case shushi, rentai:
				return "くる"
			case katei:
				return "くれ"
			case meirei:
				return "こい"
			}
		}
	case strings.HasPrefix(infType, "サ変"):
		switch conjug {
		case mizen, renyo, renyoTe:
			return "し"
		case shushi, rentai:
			return "する"
		case katei:
			return "すれ"
		case meirei:
			return "しろ"
		}
	}

	return token.Surface
}

func getCharFromColumnRow(c rune, r rune) string {
	// Unicode seq is more complicated than expected sad
	var ret rune
	switch c {
	case 'ナ', 'マ', 'ラ':
		ret = c + r
	case 'ア', 'カ', 'サ':
		ret = c + 2*r
	case 'タ':
		if r < 2 {
			ret = c + 2*r
		} else {
			ret = c + 2*r + 1 // 'ッ'
		}
	case 'ハ':
		ret = c + 3*r
	case 'ヤ':
		if r == 1 {
			ret = 'イ'
		} else if r == 3 {
			ret = 'エ'
		} else {
			ret = c + r/2
		}
	case 'ワ':
		switch r {
		case 0:
			ret = 'ワ'
		default:
			ret = 'ア' + 2*r
		}
	}
	return string(ret - 96)
}
