package nip51

import (
	"encoding/json"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
)

type ListEventKind int

const (
	MuteList        = 10000
	PinnedNotes     = 10001
	Bookmarks       = 10003
	Communities     = 10004
	PublicChats     = 10005
	BlockedRelays   = 10006
	SearchRelays    = 10007
	SimpleGroups    = 10009
	Interests       = 10015
	Emojis          = 10030
	DMRelays        = 10050
	GoodWikiAuthors = 10101
	GoodWikiRelays  = 10102
)

const (
	FollowSets          = 30000
	RelaySets           = 30002
	BookmarkSets        = 30003
	ArticleCurationSets = 30004
	VideoCurationSets   = 30005
	KindMuteSets        = 30007
	InterestSets        = 30015
	EmojiSets           = 30030
	ReleaseArtifactSets = 30063
)

const (
	Deprecated = 30001
)

type ListEvent struct {
	ListEventKind
	Identifier  string
	Title       string
	Image       string
	Description string
	PubKeys     []PubKey
	Communities []Community
	References  []string
	Hashtags    []string
}

type PubKey struct {
	PubKey string
	Relay  string
	Role   string
}

type Community struct {
	Kind       string
	PubKey     string
	Identifier string
}

func ParseListEvent(event nostr.Event, args ...string) ListEvent {
	var err error

	listev := ListEvent{
		ListEventKind: ListEventKind(event.Kind),
	}

	populateFromTags(&listev, event.Tags)

	var pk string = ""
	var sk string = ""
	if len(args) == 2 {
		pk = args[0]
		sk = args[1]
	}
	if pk != "" && sk != "" && event.Content != "" {
		var shsec []byte
		var tagsstr string
		var privtags nostr.Tags

		if shsec, err = nip04.ComputeSharedSecret(pk, sk); err != nil {
			return listev
		}
		if tagsstr, err = nip04.Decrypt(event.Content, shsec); err != nil {
			return listev
		}
		if err = json.Unmarshal([]byte(tagsstr), &privtags); err != nil {
			return listev
		}

		populateFromTags(&listev, privtags)
	}

	return listev
}

func populateFromTags(listev *ListEvent, tags nostr.Tags) {
	for _, tag := range tags {
		if len(tag) < 2 {
			continue
		}
		switch tag[0] {
		case "d":
			listev.Identifier = tag[1]
		case "title":
			listev.Title = tag[1]
		case "image":
			listev.Image = tag[1]
		case "description":
			listev.Description = tag[1]
		case "p":
			if nostr.IsValid32ByteHex(tag[1]) {
				pk := PubKey{
					PubKey: tag[1],
				}
				if len(tag) > 2 {
					pk.Relay = tag[2]
					if len(tag) > 3 {
						pk.Role = tag[3]
					}
				}
				listev.PubKeys = append(listev.PubKeys, pk)
			}
		case "a":
			k := strings.Split(tag[1], ":")
			if len(k) != 3 {
				continue
			}
			listev.Communities = append(listev.Communities, Community{
				Kind:       k[0],
				PubKey:     k[1],
				Identifier: k[2],
			})
		case "r":
			listev.References = append(listev.References, tag[1])
		case "t":
			listev.Hashtags = append(listev.Hashtags, tag[1])
		}
	}
}

func (listev ListEvent) ToHashtags() nostr.Tags {
	tags := make(nostr.Tags, 0, 26)
	tags = append(tags, nostr.Tag{"d", listev.Identifier})
	if listev.Title != "" {
		tags = append(tags, nostr.Tag{"title", listev.Title})
	}
	if listev.Image != "" {
		tags = append(tags, nostr.Tag{"image", listev.Title})
	}

	for _, pk := range listev.PubKeys {
		tags = append(tags, nostr.Tag{"p", pk.PubKey, pk.Relay, pk.Role})
	}
	for _, co := range listev.Communities {
		tags = append(tags, nostr.Tag{"a", co.Kind, co.PubKey, co.Identifier})
	}
	for _, reference := range listev.References {
		tags = append(tags, nostr.Tag{"r", reference})
	}
	for _, hashtag := range listev.Hashtags {
		tags = append(tags, nostr.Tag{"t", hashtag})
	}

	return tags
}
