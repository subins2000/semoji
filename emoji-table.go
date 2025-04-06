package main

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

type Emoji struct {
	Emoji    string `json:"emoji"`
	Name     string `json:"name"`
	Category int    `json:"category"`
}

type SearchResult struct {
	Symbol string
	Score  int // lower is better
}

var emojiTable = []Emoji{
	// {
	// 	Symbol:      "😀",
	// 	Keywords:    []string{"smile", "happy", "joy", "grin"},
	// },
}

var emojiNames []string

func populateEmojiTable() {
	file, err := os.ReadFile("emojis.json")
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(file, &emojiTable); err != nil {
		panic(err)
	}

	emojiNames = make([]string, len(emojiTable))

	for i, emoji := range emojiTable {
		emojiNames[i] = strings.ToLower(emoji.Name)
	}
}

func getEmojiSuggestions(ctx context.Context, channel chan<- []Emoji, query string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			results := []Emoji{}

			matches := fuzzy.RankFindFold(query, emojiNames)

			sort.Sort(matches)

			for _, match := range matches {
				results = append(results, emojiTable[match.OriginalIndex])
			}

			channel <- results
			// close(channel)
		}
	}
}
