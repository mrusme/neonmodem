package systemtest

import (
	"math/rand/v2"
	"strings"
)

var words = []string{
	"amber", "anchor", "apple", "arrow", "autumn", "basket", "beacon", "birch",
	"blanket", "bridge", "bronze", "candle", "canyon", "cedar", "chalk",
	"cinder", "clover", "cobalt", "copper", "coral", "cotton", "crimson",
	"crystal", "dawn", "delta", "desert", "dune", "echo", "ember", "falcon",
	"feather", "fern", "field", "flint", "forest", "fossil", "garden",
	"glacier", "granite", "harbor", "hazel", "heron", "hollow", "island",
	"ivory", "jasper", "juniper", "kettle", "lagoon", "lantern", "lemon",
	"linen", "maple", "marble", "meadow", "mirror", "moss", "nectar", "nickel",
	"oak", "ocean", "olive", "orchard", "pebble", "pepper", "pine", "plume",
	"quarry", "quartz", "rain", "raven", "reed", "ridge", "river", "saddle",
	"sage", "salt", "shadow", "shell", "silver", "slate", "spruce", "stone",
	"summit", "thistle", "thunder", "timber", "tulip", "valley", "velvet",
	"willow", "window", "winter", "yarrow", "zephyr",
}

func RandomText(n int) string {
	text := draw(n)
	for attempt := 0; attempt < 100 && distinctChars(text) < 12; attempt++ {
		text = draw(n)
	}
	return text
}

func draw(n int) string {
	if n <= 0 {
		return ""
	}
	order := rand.Perm(len(words))
	picked := make([]string, n)
	for i := range picked {
		picked[i] = words[order[i%len(order)]]
	}
	text := strings.Join(picked, " ")
	return strings.ToUpper(text[:1]) + text[1:]
}

func distinctChars(text string) int {
	chars := map[rune]bool{}
	for _, r := range text {
		chars[r] = true
	}
	return len(chars)
}
