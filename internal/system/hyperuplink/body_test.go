package hyperuplink

import (
	"testing"

	"github.com/mrusme/neonmodem/internal/system/hyperuplink/api"
	"github.com/mrusme/neonmodem/internal/system/text"
)

func TestDescribeLeavesAPlainBodyAlone(t *testing.T) {
	if got := describe("https://board.example", "Hello.\n", nil, nil); got != "Hello.\n" {
		t.Errorf("got %q", got)
	}
	if got := describe("https://board.example", "", nil, []api.Attachment{}); got != "" {
		t.Errorf("an empty attachment list changed the body: %q", got)
	}
}

func TestDescribeSummarizesAPoll(t *testing.T) {
	poll := &api.Poll{
		Options: []api.PollOption{
			{Index: 0, Text: "QWERTY", Votes: 7, Percent: 58},
			{Index: 1, Text: "Colemak", Votes: 4, Percent: 33},
			{Index: 2, Text: "Other", Votes: 1, Percent: 8},
		},
		Total:  12,
		EndsAt: "2026-10-12T18:00:00Z",
	}
	ends := text.Time(poll.EndsAt).Local().Format(pollTimeLayout)

	want := "Which layout?\n\nPoll, 12 votes, open until " + ends + ":\n" +
		"- QWERTY: 7 votes (58%)\n- Colemak: 4 votes (33%)\n- Other: 1 vote (8%)"
	if got := describe("https://board.example", "Which layout?\n\n", poll, nil); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}

	poll.Ended = true
	if got := describe("https://board.example", "x", poll, nil); got[:24] != "x\n\nPoll, 12 votes, ended" {
		t.Errorf("an ended poll reads %q", got)
	}

	open := &api.Poll{Options: []api.PollOption{{Text: "Yes", Votes: 1, Percent: 100}}, Total: 1}
	if got := describe("https://board.example", "x", open, nil); got != "x\n\nPoll, 1 vote:\n- Yes: 1 vote (100%)" {
		t.Errorf("a poll without an end reads %q", got)
	}
}

func TestDescribeWritesImageAttachmentsInImageSyntax(t *testing.T) {
	attachments := []api.Attachment{
		{ID: "a1", Filename: "my layout.png", MimeType: "image/png", URL: "/api/v1/attachments/a1"},
		{ID: "a2", Filename: "notes&more.txt", URL: "https://files.example/a2"},
		{ID: "a3", URL: "/api/v1/attachments/a3?v=2"},
		{ID: "a4", Filename: "my [draft]\\v2\nfinal.webp", MimeType: "image/webp", URL: "/api/v1/attachments/a4"},
		{ID: "a5", Filename: "report.pdf", MimeType: "application/pdf", URL: "/api/v1/attachments/a5"},
	}

	want := "Attachments:\n" +
		"- ![my layout.png (image/png)](https://board.example:3001/api/v1/attachments/a1)\n" +
		"- notes&more.txt: https://files.example/a2\n" +
		"- : https://board.example:3001/api/v1/attachments/a3?v=2\n" +
		"- ![my \\[draft\\]\\\\v2 final.webp (image/webp)](https://board.example:3001/api/v1/attachments/a4)\n" +
		"- report.pdf (application/pdf): https://board.example:3001/api/v1/attachments/a5"
	if got := describe("https://board.example:3001/", "", nil, attachments); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
