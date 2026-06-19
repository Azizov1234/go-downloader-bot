package instagram

import "os"

type Cookies struct {
	Use  bool
	File string
}

func (c Cookies) Args() []string {
	if !c.Use || c.File == "" {
		return nil
	}
	if _, err := os.Stat(c.File); err != nil {
		return nil
	}
	return []string{"--cookies", c.File}
}
