package goshots

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

func ServerInit(r *mux.Router, providers ...Provider) error {

	localRoot := os.ExpandEnv("$GOPATH/src/github.com/garbotron/goshots")
	localTemplateRoot = localRoot + "/templates"
	localStaticRoot = localRoot + "/static"

	for _, provider := range providers {
		if err := provider.Load(); err != nil {
			return err
		}

		subdomain := provider.Subdomain()
		var s *mux.Router
		if subdomain == "" {
			s = r.Host("{sub:(www.)?}" + domainName).Subrouter()
		} else {
			s = r.Host(subdomain + "." + domainName).Subrouter()
		}

		// serve files under /static using a standard file system server
		s.Handle("/static/{path:.*}", http.StripPrefix("/static/", http.FileServer(http.Dir(localStaticRoot))))

		// all other URLs go to the custom handler
		s.HandleFunc("/{file:.*}", getHandler(provider))
	}

	return nil
}

type RendererData struct {
	Provider     Provider
	StaticRoot   string
	Request      *http.Request
	Writer       http.ResponseWriter
	Filters      []Filter
	FilterValues FilterValues
}

func RenderTemplate(name string, w io.Writer, data interface{}) error {

	templatePath := path.Join(localTemplateRoot, name)
	if t, err := template.ParseFiles(templatePath); err != nil {
		return err
	} else {
		t.Execute(w, data)
		return nil
	}
}

const domainName = "gamershots.info"
const webRoot = "/"
const webStaticRoot = "/static/"

var errGoshotsPageNotFound = errors.New("Page not found")
var localTemplateRoot string
var localStaticRoot string

func renderPage(
	provider Provider,
	pageFileName string,
	r *http.Request,
	w http.ResponseWriter) error {

	filters := provider.Filters()

	filterValues, err := getFiltersCookieValue(provider, r)

	if err != nil || len(filterValues) != len(filters) {
		// the settings were invalid - leave the filter presence all false and construct a blank set
		filterValues = make(FilterValues, len(filters))
		for i := range filterValues {
			filterValues[i].Enabled = false
		}
	}

	// fill in the defaults for all disabled options
	for i := range filterValues {
		if !filterValues[i].Enabled {
			filterValues[i].Values = filters[i].DefaultValues()
		}
	}

	data := RendererData{
		Provider:     provider,
		StaticRoot:   webStaticRoot,
		Request:      r,
		Writer:       w,
		Filters:      filters,
		FilterValues: filterValues,
	}

	switch strings.ToLower(pageFileName) {

	case "":
		fallthrough
	case "main":
		return RenderMainPage(&data)

	case "filters":
		return RenderFiltersPage(&data, false)

	case "scrape":
		return RenderScrapePage(&data)

	case "about":
		return RenderTemplate("about.goshots", data.Writer, &data)

	case "donate":
		return RenderTemplate("donate.goshots", data.Writer, &data)

	default:
		return errGoshotsPageNotFound
	}
}

func getFiltersCookieValue(provider Provider, r *http.Request) (FilterValues, error) {
	cookie, err := r.Cookie(fmt.Sprintf("%s_filters", provider.ShortName()))
	if err != nil {
		return nil, err
	}

	cookieText, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return nil, err
	}

	data := FilterValues{}
	err = json.Unmarshal([]byte(cookieText), &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func getHandler(provider Provider) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		idx := strings.LastIndex(r.URL.Path, "/")
		file := r.URL.Path[idx+1:]

		if err := renderPage(provider, file, r, w); err != nil {
			if err == errGoshotsPageNotFound {
				http.NotFound(w, r)
			} else {
				http.Error(w, err.Error(), 500)
			}
		}
	}
}
