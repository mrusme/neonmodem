package adapter

type Capabilities []Capability

type Capability struct {
	ID   string
	Name string
}

func (caps *Capabilities) IsCapableOf(capability string) bool {
	var can bool = false

	for _, capb := range *caps {
		if capb.ID == capability {
			can = true
			break
		}
	}

	return can
}
