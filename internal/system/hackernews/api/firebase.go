package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
)

var ErrNotFound = errors.New("item not found")

var Lists = map[string]string{
	"top":  "topstories",
	"new":  "newstories",
	"best": "beststories",
	"ask":  "askstories",
	"show": "showstories",
	"jobs": "jobstories",
}

type Item struct {
	ID          int    `json:"id"`
	Deleted     bool   `json:"deleted"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Text        string `json:"text"`
	Dead        bool   `json:"dead"`
	Parent      int    `json:"parent"`
	Kids        []int  `json:"kids"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Title       string `json:"title"`
	Descendants int    `json:"descendants"`
}

func (c *Client) Stories(ctx context.Context, list string) ([]int, error) {
	endpoint, ok := Lists[list]
	if !ok {
		return nil, fmt.Errorf("unknown Hacker News list %q", list)
	}

	var ids []int
	if err := c.firebase.Get(ctx, "/"+endpoint+".json", nil, &ids); err != nil {
		return nil, err
	}

	return ids, nil
}

func (c *Client) Item(ctx context.Context, id int) (*Item, error) {
	var item *Item
	if err := c.firebase.Get(ctx, "/item/"+strconv.Itoa(id)+".json", nil, &item); err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("item %d: %w", id, ErrNotFound)
	}

	return item, nil
}

func (c *Client) Items(ctx context.Context, ids []int) ([]*Item, error) {
	items := make([]*Item, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	sem := make(chan struct{}, c.workers)

	for i, id := range ids {
		wg.Add(1)
		go func(i int, id int) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			defer func() { <-sem }()

			items[i], errs[i] = c.Item(ctx, id)
		}(i, id)
	}
	wg.Wait()

	var out []*Item
	var failures []error
	for i := range items {
		if errs[i] != nil {
			failures = append(failures, errs[i])
			continue
		}
		out = append(out, items[i])
	}

	if len(out) == 0 && len(failures) > 0 {
		return nil, failures[0]
	}

	return out, nil
}

func (c *Client) Descendants(ctx context.Context, root *Item) (map[int]*Item, error) {
	found := make(map[int]*Item)
	queue := append([]int(nil), root.Kids...)

	for len(queue) > 0 {
		items, err := c.Items(ctx, queue)
		if err != nil {
			return found, err
		}

		queue = queue[:0]
		for _, item := range items {
			found[item.ID] = item
			queue = append(queue, item.Kids...)
		}
	}

	return found, nil
}
