package goshots

import (
	"errors"
	"html/template"
)

type FilterType int

const (
	FilterTypeNumber      FilterType = iota
	FilterTypeNumberRange FilterType = iota
	FilterTypeSelectOne   FilterType = iota
	FilterTypeSelectMany  FilterType = iota
)

type Filter interface {
	Name() string
	Prompt() string
	Type() FilterType
	Names(provider Provider) ([]string, error)
	DefaultValues() []int
}

type FilterValue struct {
	Enabled bool
	Values  []int
}

// a filter selection is saved as a mapping of [filter index] => [values for that filter]
type FilterValues []FilterValue

type ScraperContext interface {
	Log(fmt string, a ...interface{})
	Error(context string, err error)
	Done(err error)
}

type Scraper interface {
	Abort()
	Progress() (stage string, cur int, total int)
}

var scraperAbortError = errors.New("scrape operation aborted")
var elemNotFoundError = errors.New("element not found")

func ScraperAbortError() error {
	return scraperAbortError
}

func IsScraperAbortError(err error) bool {
	return (err == scraperAbortError)
}

func ElemNotFoundError() error {
	return elemNotFoundError
}

func IsElemNotFoundError(err error) bool {
	return (err == elemNotFoundError)
}

type Provider interface {
	ShortName() string
	PrettyName() string
	Description() []template.HTML
	Title() string
	Prompt() string
	Filters() []Filter
	Load() error
	RandomElem(filterValues *FilterValues) (interface{}, error)
	ElemSolution(elem interface{}) (string, error)
	RenderContentHtml(elem interface{}) (template.HTML, error)
	CanScrape() bool
	StartScraping(context ScraperContext) Scraper
}
