package instagram

const Platform = "instagram"

type Provider struct{}

func NewProvider() Provider {
	return Provider{}
}

func (Provider) Parse(raw string) (ParsedURL, error) {
	return Parse(raw)
}
