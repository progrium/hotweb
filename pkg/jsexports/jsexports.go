package jsexports

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const eof = rune(-1)

type itemType string

var keywords = []string{
	"as",
	"let",
	"const",
	"var",
	"function",
	"class",
	"default",
}

const (
	itemError      itemType = "err"
	itemEOF        itemType = "eof"
	itemExport     itemType = "export"
	itemIdentifier itemType = "ident"
	itemText       itemType = "text"
	itemString     itemType = "string"
	itemKeyword    itemType = "keyword"
	itemNumber     itemType = "number"
)

type item struct {
	typ itemType
	val string
}

func (i item) String() string {
	switch i.typ {
	case itemEOF:
		return "EOF"
	case itemError:
		return i.val
	}
	return fmt.Sprintf("%q %s", i.val, i.typ)
}

type stateFn func(*lexer) stateFn

type lexer struct {
	name  string
	input string
	start int
	pos   int
	width int
	state stateFn
	items chan item

	isFunc bool
}

func lex(name, input string) *lexer {
	l := &lexer{
		name:  name,
		input: input,
		state: lexText,
		items: make(chan item, 2),
	}
	return l
}

func (l *lexer) nextItem() item {
	for {
		select {
		case item := <-l.items:
			return item
		default:
			l.state = l.state(l)
		}
	}
}

func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.input[l.start:l.pos]}
	l.start = l.pos
}

func (l *lexer) ignore() {
	l.start = l.pos
}

func (l *lexer) backup() {
	l.pos -= l.width
}

func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

func (l *lexer) peekNextWord() string {
	oldStart := l.start
	oldPos := l.pos
	for strings.IndexRune(" ", l.next()) >= 0 {
	}
	l.backup()
	l.start = l.pos
	for isAlphaNumeric(l.next()) {
	}
	l.backup()
	ident := l.input[l.start:l.pos]
	l.start = oldStart
	l.pos = oldPos
	return ident
}

func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

func (l *lexer) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

func (l *lexer) next() (r rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{
		itemError,
		fmt.Sprintf(format, args...),
	}
	return nil
}

func lexText(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], "export") {
			if l.pos > l.start {
				l.emit(itemText)
			}
			return lexExport
		}
		if l.next() == eof {
			break
		}
	}
	if l.pos > l.start {
		l.emit(itemText)
	}
	l.emit(itemEOF)
	return nil
}

func lexNumber(l *lexer) stateFn {
	l.accept("+-")
	digits := "0123456789"
	l.acceptRun(digits)
	if l.accept(".") {
		l.acceptRun(digits)
	}
	if isAlphaNumeric(l.peek()) {
		l.next()
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	l.emit(itemNumber)
	return lexInsideExport
}

func lexExport(l *lexer) stateFn {
	l.pos += len("export")
	l.emit(itemExport)
	l.isFunc = false
	return lexInsideExport
}

func lexInsideExport(l *lexer) stateFn {
	for {
		switch r := l.next(); {
		case r == ';', r == '\n', r == '=':
			l.backup()
			return lexText
		case r == eof:
			return l.errorf("unexpected end of export statement")
		case unicode.IsSpace(r), r == ',':
			l.ignore()
		case r == '(':
			return lexQuoted(l, ')', lexInsideExport)
		case r == '"':
			return lexQuoted(l, '"', lexInsideExport)
		case r == '`':
			return lexQuoted(l, '`', lexInsideExport)
		case r == '\'':
			return lexQuoted(l, '\'', lexInsideExport)
		case r == '{':
			if l.isFunc {
				l.backup()
				return lexText
			}
			return lexInsideBraces
		case isAlphaNumeric(r):
			l.backup()
			return lexIdentifier(l, lexInsideExport)
		}
	}
}

func lexInsideBraces(l *lexer) stateFn {
	foundColon := false
	for {
		switch r := l.next(); {
		case r == '}':
			return lexText
		case r == eof:
			return l.errorf("unexpected end of braced block")
		case unicode.IsSpace(r), r == ',', r == '=':
			l.ignore()
		case r == ':':
			foundColon = true
			l.ignore()
		case r == '"':
			return lexQuoted(l, '"', lexInsideBraces)
		case r == '`':
			return lexQuoted(l, '`', lexInsideBraces)
		case r == '\'':
			return lexQuoted(l, '\'', lexInsideBraces)
		case isAlphaNumeric(r):
			if foundColon {
				foundColon = false
				for isAlphaNumeric(l.next()) {
				}
				break
			}
			l.backup()
			return lexIdentifier(l, lexInsideBraces)
		}
	}
}

func lexIdentifier(l *lexer, nextState stateFn) stateFn {
	return func(l *lexer) stateFn {
		for isAlphaNumeric(l.next()) {
		}
		l.backup()
		isKeyword := false
		for _, keyword := range keywords {
			if l.input[l.start:l.pos] == keyword {
				if keyword == "function" {
					l.isFunc = true
				}
				l.emit(itemKeyword)
				isKeyword = true
			}
		}
		if !isKeyword {
			if l.peekNextWord() != "as" {
				l.emit(itemIdentifier)
			}
		}
		return nextState
	}
}

func lexQuoted(l *lexer, quote rune, nextState stateFn) stateFn {
	return func(l *lexer) stateFn {
		for {
			switch r := l.next(); {
			case r == quote:
				l.emit(itemString)
				return nextState
			case r == eof || r == '\n':
				return l.errorf("unexpected end of quoted string")
			}
		}
	}
}

func isAlphaNumeric(r rune) bool {
	if unicode.IsDigit(r) || unicode.IsLetter(r) {
		return true
	}
	return false
}

func Exports(src []byte) ([]string, error) {
	l := lex("", string(src))
	i := l.nextItem()
	set := make(map[string]struct{})
	for i.typ != itemEOF {
		if i.typ == itemIdentifier {
			set[strings.Trim(i.val, "{}()-_;,.$!")] = struct{}{}
		}
		i = l.nextItem()
	}
	var exports []string
	for n, _ := range set {
		exports = append(exports, n)
	}
	return exports, nil
}
