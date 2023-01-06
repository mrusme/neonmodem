package forum

import "strings"

type Forum struct {
	ID   string
	Name string

	Info string

	SysIDX int
}

func (forum Forum) FilterValue() string {
	return forum.Name
}

func (forum Forum) Title() string {
	return strings.Title(forum.Name)
}

func (forum Forum) Description() string {
	return forum.Info
}
