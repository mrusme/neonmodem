package config

import (
	"os"
	"slices"
	"strings"
	"testing"
)

const openWithFile = `
[[OpenWith]]
name = 'Save page'
cmd = 'wget -q -P ~/Downloads "$NM_POST_URL"'

[[OpenWith]]
Name = 'Bookmark'
Cmd = 'nb bookmark "$NM_POST_URL" --tags incoming'
`

func TestOpenWithSurvivesASave(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, openWithFile)

	cfg, err := LoadFrom([]string{path}, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []OpenWith{
		{Name: "Save page", Cmd: `wget -q -P ~/Downloads "$NM_POST_URL"`},
		{Name: "Bookmark", Cmd: `nb bookmark "$NM_POST_URL" --tags incoming`},
	}
	if !slices.Equal(cfg.OpenWith, want) {
		t.Fatalf("got %+v", cfg.OpenWith)
	}

	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[[OpenWith]]") || !strings.Contains(string(data), "name = 'Bookmark'") {
		t.Errorf("unexpected file:\n%s", data)
	}

	again, err := LoadFrom([]string{path}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(again.OpenWith, want) {
		t.Errorf("after a save: %+v", again.OpenWith)
	}
}

func TestOpenWithIsLeftOutWhenEmpty(t *testing.T) {
	cfg := Defaults("/cache")
	doc, err := cfg.Document()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc), "OpenWith") {
		t.Errorf("an empty list was written:\n%s", doc)
	}
}

func TestValidOpenWithLeavesOutIncompleteEntries(t *testing.T) {
	cfg := Defaults("/cache")
	cfg.OpenWith = []OpenWith{
		{Name: "  Save page ", Cmd: "wget x"},
		{Name: "No command", Cmd: "  "},
		{Name: "", Cmd: "true"},
	}

	valid, problems := cfg.ValidOpenWith()
	if !slices.Equal(valid, []OpenWith{{Name: "Save page", Cmd: "wget x"}}) {
		t.Errorf("got %+v", valid)
	}
	if len(problems) != 2 || !strings.Contains(problems[0], "entry 2") || !strings.Contains(problems[1], "entry 3") {
		t.Errorf("got %q", problems)
	}
}
