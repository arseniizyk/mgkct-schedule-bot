package models

import "errors"

var (
	ErrGroupNotFound   = errors.New("group not found")
	ErrScraperInternal = errors.New("scraper error")
	ErrBadInput        = errors.New("bad input")
)
