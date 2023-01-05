package forum

type Forum struct {
	ID   string
	Name string

	SysIDX int
}

func (forum Forum) FilterValue() string {
	return forum.Name
}

func (forum Forum) Title() string {
	return forum.Name
}

func (forum Forum) Description() string {
	return forum.ID
}
