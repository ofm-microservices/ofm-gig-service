package service

import "errors"

var (
	ErrNilGigRepository = errors.New("gig repository is nil")
	ErrNilFileService   = errors.New("file service is nil")
	ErrNilEventBroker   = errors.New("event broker is nil")
	ErrNilSlugger       = errors.New("slugger is nil")
	ErrNilLogger        = errors.New("logger is nil")
)
