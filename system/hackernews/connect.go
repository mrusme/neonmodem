package hackernews

func (sys *System) Connect(sysURL string) error {
	// Credentials
	credentials := make(map[string]string)
	credentials["username"] = ""
	credentials["password"] = ""

	if sys.config == nil {
		sys.config = make(map[string]interface{})
	}
	sys.config["credentials"] = credentials

	return nil
}
