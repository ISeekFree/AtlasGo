package auth

type Request struct {
	Token      string
	Domain     string
	AppTag     string
	IP         string
	Admin      bool
	Attributes map[string]any
}

func (r Request) Attribute(key string) any {
	if r.Attributes == nil {
		return nil
	}
	return r.Attributes[key]
}
