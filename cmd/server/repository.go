package main

type urlRepository interface {
	save(record urlRecord) error
	get(code string) (urlRecord, bool)
}

var _ urlRepository = (*memoryStore)(nil)
