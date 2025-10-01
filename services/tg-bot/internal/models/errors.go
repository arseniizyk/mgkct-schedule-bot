package models

import "errors"

var (
	ErrGroupNotFound   = errors.New("group not found")
	ErrUserNoGroup     = errors.New("no user's group")
	ErrInternal        = errors.New("internal error")
	ErrScraperInternal = errors.New("scraper error")
	ErrBadInput        = errors.New("bad input")
)
