package service

import "github.com/gosimple/slug"

type slugger struct{}

// NewSlugger constructs the application slug generator used to normalize gig
// titles before persistence.
func NewSlugger() Slugger {
	return slugger{}
}

func (slugger) Generate(title, gigID string) string {
	base := slug.Make(title)
	if base == "" {
		return ""
	}
	if gigID == "" {
		return ""
	}
	return base + "-" + gigID
}
