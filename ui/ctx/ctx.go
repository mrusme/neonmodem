package ctx

import "github.com/mrusme/gobbs/system"

type Ctx struct {
	Screen  [2]int
	Content [2]int
	Systems []*system.System
	Loading bool
}

func New() Ctx {
	return Ctx{
		Screen:  [2]int{0, 0},
		Content: [2]int{0, 0},
		Loading: false,
	}
}

func (c *Ctx) AddSystem(sys *system.System) error {
	c.Systems = append(c.Systems, sys)
	return nil
}
