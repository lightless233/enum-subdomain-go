package enumsubdomain

import "math/rand"

const LETTERS = "abcdefghijklmnopqrstuvwxyz0123456789"

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = LETTERS[rand.Intn(len(LETTERS))]
	}
	return string(b)
}

func BuildAlphaTable() []string {
	table := make([]string, 0, 37)
	for i := 97; i < 123; i++ {
		table = append(table, string(rune(i)))
	}
	for i := 48; i < 58; i++ {
		table = append(table, string(rune(i)))
	}
	table = append(table, "-")
	return table
}
