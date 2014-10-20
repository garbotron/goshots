package animeclips

import (
	"errors"
	"github.com/PuerkitoBio/goquery"
	"github.com/garbotron/goshots/core"
	"html/template"
	"math/rand"
	"net/url"
	"strings"
)

type Animeclips struct {
	lines []string
}

func (_ *Animeclips) ShortName() string {
	return "animeclips"
}

func (_ *Animeclips) PrettyName() string {
	return "Animeclips"
}

func (_ *Animeclips) Description() []template.HTML {
	return []template.HTML{
		"Animeclips is a drinking game for anime dorks.",
		"Check out a quick clip of an opening on Youtube, see if you can name the show!",
		"<b>If you can't name the show, take a drink!</b>",
		"<b>If you can, everyone else takes a drink and you go again!</b>",
	}
}

func (_ *Animeclips) Title() string {
	return "Animeclips!"
}

func (_ *Animeclips) Prompt() string {
	return "What show is this?"
}

func (_ *Animeclips) CanScrape() bool {
	return false
}

func (_ *Animeclips) StartScraping(context goshots.ScraperContext) goshots.Scraper {
	return nil
}

func (_ *Animeclips) Filters() []goshots.Filter {
	return []goshots.Filter{}
}

func (_ *Animeclips) Load() error {
	return nil
}

type solution struct {
	name string
	url  string
}

func (_ *Animeclips) RandomElem(filterValues *goshots.FilterValues) (interface{}, error) {

	// we want this format:
	//http://www.youtube.com/v/hHsvnJ1NqO8&start=40&end=46&version=3&autoplay=1

	idxs := []int{}
	for i, s := range AllAnime {
		if strings.Contains(strings.ToLower(s), "movie") ||
			strings.Contains(strings.ToLower(s), "special") {
			continue
		}
		idxs = append(idxs, i)
	}

	idx := idxs[rand.Int()%len(idxs)]
	str := AllAnime[idx]

	parenIdx := strings.Index(str, "(")
	if parenIdx < 0 {
		return nil, errors.New("Couldn't find open paren")
	}

	animeName := strings.TrimSpace(str[0:parenIdx])
	searchTxt := url.QueryEscape(animeName)
	searchUrl := "http://www.youtube.com/results?search_query=" + searchTxt + "+op&lclk=short"

	doc, err := goquery.NewDocument(searchUrl)
	if err != nil {
		return nil, err
	}

	link, ok := doc.Find(".yt-lockup-content a[href^='/watch?v=']").First().Attr("href")
	if !ok {
		return nil, errors.New("Couldn't find watch link")
	}
	searchId := link[len("/watch?v="):]
	fullUrl := "http://www.youtube.com/v/" + searchId + "&start=40&end=46&autoplay=1?rel=0&showinfo=0"

	return &solution{animeName, fullUrl}, nil
}

func (_ *Animeclips) ElemSolution(elem interface{}) (string, error) {
	sol := elem.(*solution)
	return sol.name, nil
}

func (_ *Animeclips) RenderContentHtml(elem interface{}) (template.HTML, error) {
	sol := elem.(*solution)
	str := `<iframe src="` + sol.url + `" frameborder="0" width="100%" height="100%"></iframe>`
	return template.HTML(str), nil
}
