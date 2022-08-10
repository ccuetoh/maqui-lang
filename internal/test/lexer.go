package test

import (
	"math/rand"
	"strings"
)

const validTokens = "func;main;(;);{;};\"this is a string\";\"this is a longer string containing a bunch of text: Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.\";\"this is a small string\";\"\";+;:=;-;123;321;//comment\n;\n"

func GetRandomTokens(size int) string {
	return GetRandomTokensWithSep(size, " ")
}

func GetRandomTokensWithSep(size int, sep string) string {
	valid := strings.Split(validTokens, ";")

	var toks []string
	for len(toks) < size {
		toks = append(toks, valid[rand.Intn(len(valid))])
	}

	return strings.Join(toks, sep)
}
