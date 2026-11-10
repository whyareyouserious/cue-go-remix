// Package textnorm normalizes Chinese speech transcripts for display and storage.
package textnorm

import (
	_ "embed"
	"strings"
)

// 字典原文及来源、许可证见 data/README.md；使用 OpenCC t2s 的词组优先规则。
//
//go:embed data/TSCharacters.txt
var characters string

//go:embed data/TSPhrases.txt
var phrases string

type node struct {
	next  map[rune]*node
	value string
}

var simplified = newDictionary(characters, phrases)

func newDictionary(dictionaries ...string) *node {
	root := &node{next: make(map[rune]*node)}
	for _, dictionary := range dictionaries {
		for _, line := range strings.Split(dictionary, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
				continue
			}
			current := root
			for _, r := range fields[0] {
				child := current.next[r]
				if child == nil {
					child = &node{next: make(map[rune]*node)}
					current.next[r] = child
				}
				current = child
			}
			// OpenCC 字典多个候选以空格分隔，t2s 使用首选值。
			current.value = fields[1]
		}
	}
	return root
}

// Simplified converts traditional Chinese to simplified Chinese using OpenCC's
// t2s dictionaries. It preserves names covered by phrase exceptions and leaves
// non-Chinese text unchanged. It does not correct recognition errors or rewrite
// regional vocabulary. The dictionary is immutable and safe for concurrent use.
func Simplified(text string) string {
	input := []rune(text)
	var output strings.Builder
	output.Grow(len(text))
	for at := 0; at < len(input); {
		current := simplified
		end, replacement := at, ""
		for i := at; i < len(input); i++ {
			current = current.next[input[i]]
			if current == nil {
				break
			}
			if current.value != "" {
				end, replacement = i+1, current.value
			}
		}
		if end == at {
			output.WriteRune(input[at])
			at++
		} else {
			output.WriteString(replacement)
			at = end
		}
	}
	return output.String()
}
