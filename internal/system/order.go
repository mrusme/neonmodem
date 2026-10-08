package system

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/mrusme/neonmodem/internal/models/post"
)

type Order string

const (
	OrderNew      Order = "new"
	OrderActive   Order = "active"
	OrderHot      Order = "hot"
	OrderTopDay   Order = "top-day"
	OrderTopWeek  Order = "top-week"
	OrderTopMonth Order = "top-month"
	OrderTopYear  Order = "top-year"
	OrderTopAll   Order = "top-all"
	OrderComments Order = "comments"
)

func AllOrders() []Order {
	return []Order{
		OrderNew, OrderActive, OrderHot,
		OrderTopDay, OrderTopWeek, OrderTopMonth, OrderTopYear, OrderTopAll,
		OrderComments,
	}
}

func TopOrders() []Order {
	return []Order{OrderTopDay, OrderTopWeek, OrderTopMonth, OrderTopYear, OrderTopAll}
}

func ParseOrder(s string) (Order, error) {
	order := Order(strings.ToLower(strings.TrimSpace(s)))
	if slices.Contains(AllOrders(), order) {
		return order, nil
	}

	keys := make([]string, 0, len(AllOrders()))
	for _, o := range AllOrders() {
		keys = append(keys, string(o))
	}
	return OrderNew, fmt.Errorf("unknown sort order %q; known orders: %s",
		s, strings.Join(keys, ", "))
}

func (o Order) Label() string {
	switch o {
	case OrderNew:
		return "New"
	case OrderActive:
		return "Active"
	case OrderHot:
		return "Hot"
	case OrderTopDay:
		return "Top: today"
	case OrderTopWeek:
		return "Top: past week"
	case OrderTopMonth:
		return "Top: past month"
	case OrderTopYear:
		return "Top: past year"
	case OrderTopAll:
		return "Top: all time"
	case OrderComments:
		return "Most comments"
	}
	return string(o)
}

func (o Order) Info() string {
	switch o {
	case OrderNew:
		return "Newest posts first"
	case OrderActive:
		return "Posts with the latest replies first"
	case OrderHot:
		return "Each site's own trending order"
	case OrderTopDay:
		return "Highest score of the past day"
	case OrderTopWeek:
		return "Highest score of the past week"
	case OrderTopMonth:
		return "Highest score of the past month"
	case OrderTopYear:
		return "Highest score of the past year"
	case OrderTopAll:
		return "Highest score ever"
	case OrderComments:
		return "Posts with the most replies first"
	}
	return ""
}

func (o Order) Compare() func(a, b post.Post) int {
	switch o {
	case OrderNew:
		return newestFirst
	case OrderComments:
		return mostRepliesFirst
	default:
		return nil
	}
}

func (o Order) Sources() []Order {
	if o == OrderComments {
		return []Order{OrderActive, OrderHot}
	}
	return nil
}

func newestFirst(a, b post.Post) int {
	return b.CreatedAt.Compare(a.CreatedAt)
}

func mostRepliesFirst(a, b post.Post) int {
	if c := cmp.Compare(b.ReplyCount, a.ReplyCount); c != 0 {
		return c
	}
	return newestFirst(a, b)
}

type Ordering struct {
	Default   Order
	Supported []Order
}

func Only(order Order) Ordering {
	return Ordering{Default: order, Supported: []Order{order}}
}

func (o Ordering) Supports(order Order) bool {
	return slices.Contains(o.Supported, order)
}

func (o Ordering) Resolve(want Order) Order {
	if o.Supports(want) {
		return want
	}
	return o.Default
}

func (o Ordering) First(candidates []Order) Order {
	for _, c := range candidates {
		if o.Supports(c) {
			return c
		}
	}
	return o.Default
}

func (o Ordering) Without(order Order) Ordering {
	if order == o.Default {
		return o
	}
	return Ordering{
		Default: o.Default,
		Supported: slices.DeleteFunc(slices.Clone(o.Supported), func(s Order) bool {
			return s == order
		}),
	}
}
