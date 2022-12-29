package post

type Post struct {
	ID string

	Subject string
}

func (post Post) FilterValue() string {
	return post.Subject
}

func (post Post) Title() string {
	return post.Subject
}

func (post Post) Description() string {
	return post.ID
}
