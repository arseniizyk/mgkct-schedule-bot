package errors

import "errors"

var (
	GroupNotFound   = errors.New("group not found")
	UserNoGroup     = errors.New("no user's group")
	Internal        = errors.New("internal error")
	ScraperInternal = errors.New("scraper error")
	BadInput        = errors.New("bad input")
)
