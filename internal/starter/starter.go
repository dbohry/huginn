// Package starter holds the built-in list of feeds offered to new users.
package starter

import (
	_ "embed"
	"encoding/json"
)

// Feed is one suggested feed.
type Feed struct {
	Category string `json:"category"`
	Title    string `json:"title"`
	URL      string `json:"url"`
}

//go:embed default.json
var defaultJSON []byte

// Defaults returns the built-in list, in the order it is shown.
func Defaults() []Feed {
	var feeds []Feed
	if err := json.Unmarshal(defaultJSON, &feeds); err != nil {
		panic("starter: default.json is invalid: " + err.Error()) // checked by a test
	}
	return feeds
}
