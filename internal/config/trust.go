package config

func (c *Config) CommandsAllowed() error {
	if c.file == nil {
		return nil
	}
	return checkTrust(c.path, c.file)
}
