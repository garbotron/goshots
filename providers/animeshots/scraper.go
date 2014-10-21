package animeshots

import (
	"errors"
	"fmt"
	"github.com/garbotron/goshots/core"
	"github.com/garbotron/goshots/utils"
	"gopkg.in/mgo.v2"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const numShowScrapers = 15

func (as *Animeshots) CanScrape() bool {
	return true
}

func (as *Animeshots) StartScraping(context goshots.ScraperContext) goshots.Scraper {
	db := as.db.DB(MongoTempDbName)
	db.DropDatabase() // in case there was any stale data left over
	shows := db.C(MongoShowsCollectionName)

	s := scraper{
		cxt:         context,
		as:          as,
		listings:    make(map[string]bool),
		shows:       shows,
		proxyTarget: utils.CreateProxyTarget(20),
		stage:       "beginning scan",
		abortSignal: make(chan struct{}, 1),
	}

	go s.scrape()
	return &s
}

func (s *scraper) Abort() {
	if s.aborting {
		s.cxt.Log("already aborting...")
	} else {
		s.aborting = true
		s.abortSignal <- struct{}{}
		s.stage = "aborting"
		s.cxt.Log("aborting...")
	}
}

func (s *scraper) Progress() (stage string, cur int, total int) {
	stage = s.stage
	cur, _ = s.shows.Count()
	total = len(s.listings)
	return
}

type scraper struct {
	cxt         goshots.ScraperContext
	as          *Animeshots
	listings    map[string]bool // list of all URLs to scan (in map form to remove duplicates)
	shows       *mgo.Collection // full show data used for final result
	proxyTarget *utils.ProxyTarget
	stage       string
	aborting    bool
	abortSignal chan struct{}
}

func (s *scraper) scrape() {

	// we scrape in 2 phases:
	// 1 - look at the listing pages only (URLs)
	// 2 - for each show, scrape all the related pages

	s.cxt.Log("collecting show listings...")
	s.stage = "collecting show list"
	if err := s.scrapeShowListings(); err != nil {
		s.finish(err)
		return
	}

	s.cxt.Log("starting full scan...")
	s.stage = "scanning"
	if err := s.scrapeShows(); err != nil {
		s.finish(err)
		return
	}

	s.cxt.Log("committing changes...")
	s.stage = "committing changes"
	if err := s.commitChanges(); err != nil {
		s.finish(err)
		return
	}

	s.cxt.Log("scrape complete! shutting down...")
	s.finish(nil)
}

func (s *scraper) finish(err error) {
	s.proxyTarget.Dispose()
	s.cxt.Done(err)
}

type regexResult struct {
	groups []string
	index  int
}

var regexNoMatchError = errors.New("regex didn't match")

// get all regex matches for all numbered () subgroups within the text
func rxFindOne(hasytack string, needle string) (regexResult, error) {
	re := regexp.MustCompile(needle)
	res := re.FindStringSubmatchIndex(hasytack)
	ret := regexResult{}
	if len(res) == 0 {
		return ret, regexNoMatchError
	}
	ret.index = res[0]
	ret.groups = make([]string, (len(res)/2)-1)
	for j := range ret.groups {
		ret.groups[j] = hasytack[res[(j+1)*2]:res[((j+1)*2)+1]]
	}
	return ret, nil
}

// get the first regex matches for all numbered () subgroups within the text
func rxFindAll(hasytack string, needle string) []regexResult {
	re := regexp.MustCompile(needle)
	result := re.FindAllStringSubmatchIndex(hasytack, -1)
	ret := make([]regexResult, len(result))
	for i, res := range result {
		ret[i].index = res[0]
		ret[i].groups = make([]string, (len(res)/2)-1)
		for j := range ret[i].groups {
			ret[i].groups[j] = hasytack[res[(j+1)*2]:res[((j+1)*2)+1]]
		}
	}
	return ret
}

// performs rxFindAll() only on text that occurs after the given pattern
func rxFindAllAfter(hasytack string, needle string, after string) []regexResult {
	ret := regexp.MustCompile(after).FindStringIndex(hasytack)
	if len(ret) == 0 {
		return []regexResult{}
	}
	return rxFindAll(hasytack[ret[1]:], needle)
}

// performs rxFindAll() only on text that occurs for each section bounded by start and end patterns
func rxFindAllRegions(hasytack string, start string, end string) []regexResult {
	return rxFindAll(hasytack, start+`((?s).*?)`+end)
}

func (s *scraper) scrapeShowListings() error {
	// don't use proxy for this giant page
	doc, err := utils.DownloadPage("http://www.animeclick.it/AnimeSlide.php?year=blank&ordine=xtitjap&senso=ASC")
	if err != nil {
		return err
	}

	for _, match := range rxFindAllAfter(doc, `a href="(/anime/[^"]+)"`, `bgcolor="#303161"`) {
		s.listings[match.groups[0]] = true
	}

	s.cxt.Log("found %d shows!", len(s.listings))
	return nil
}

func (s *scraper) scrapeShows() error {
	listings := make(chan string)
	done := make(chan struct{})
	for i := 0; i < numShowScrapers; i++ {
		go s.scrapeShowsFromChannel(listings, done)
	}

	for url, _ := range s.listings {
		if s.aborting {
			break
		}
		select {
		case listings <- url:
		case <-s.abortSignal:
		}
		if s.aborting {
			break
		}
	}
	close(listings)
	for i := 0; i < numShowScrapers; i++ {
		<-done
	}

	if s.aborting {
		return goshots.ScraperAbortError()
	} else {
		return nil
	}
}

func (s *scraper) scrapeShowsFromChannel(listings <-chan string, done chan<- struct{}) {
	for url := range listings {
		if !s.scrapeShow(url) {
			break // aborted
		}
	}
	done <- struct{}{}
}

func (s *scraper) scrapeShow(listingUrl string) bool {
	if s.aborting {
		return false
	}

	s.cxt.Log("starting show: %s...", listingUrl)
	var err error

	doc := s.downloadPage(rootUrl(listingUrl))

	origTitle := findTitledData(doc, "Titolo Originale:")
	engTitle := findTitledData(doc, "Titolo Inglese:")
	format := findTitledData(doc, "Formato:")
	year := findTitledData(doc, "Anno:")
	tags := findTitledData(doc, "Genere:")

	if format == "" {
		s.cxt.Error(listingUrl, errors.New("couldn't find type"))
	}

	show := Show{}

	show.Name = engTitle
	if show.Name == "" {
		show.Name = origTitle
	}
	show.Type = s.translateType(format)
	show.Year, err = strconv.Atoi(year)
	show.HasYear = err == nil

	show.Tags = []string{}
	if tags != "" {
		for _, r := range rxFindAll(tags, `<a.*?>([^<]+)</a>`) {
			tag := s.translateTag(strings.TrimSpace(r.groups[0]))
			if tag != "" {
				show.Tags = append(show.Tags, tag)
			}
		}
	}

	show.ScreenshotUrls = []string{}
	for _, imgDiv := range rxFindAllRegions(doc, `<div class="anime_img"`, `</div>`) {
		r, err := rxFindOne(imgDiv.groups[0], `<a.*?href="([^"]+)"`)
		if err == regexNoMatchError {
			r, err = rxFindOne(imgDiv.groups[0], `<img.*?src="([^"]+)"`)
		}
		if err == nil {
			show.ScreenshotUrls = append(show.ScreenshotUrls, rootUrl(r.groups[0]))
		}
	}

	s.shows.Insert(&show)

	s.cxt.Log("completed show: %s (%d)", show.Name, show.Year)
	runtime.GC()

	return true
}

func rootUrl(url string) string {
	if !strings.HasPrefix(url, "http://") {
		url = "http://www.animeclick.it/" + strings.TrimLeft(url, "/")
	}
	return url
}

func findTitledData(doc string, title string) string {
	r, err := rxFindOne(doc, `<td class="atitle[13]"[^>]*>`+title+`</td>[ \t\r\n]*<td class="acontent[13]">((?s).*?)</td>`)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(r.groups[0])
}

func (s *scraper) translateType(format string) string {
	switch {
	case strings.Contains(format, "TV"):
		return "TV"
	case strings.Contains(format, "OAV"):
		return "OVA"
	case strings.Contains(format, "Film"):
		return "Movie"
	case strings.Contains(format, "Special"):
		return "Special"
	case strings.Contains(format, "Extra"):
		return "Extra"
	case strings.Contains(format, "Live Action"):
		return "Live Action"
	case strings.Contains(format, "Web"):
		return "Web"
	case format == "Drama":
		return "Drama"
	default:
		s.cxt.Error("main", errors.New(fmt.Sprintf("Unknown type: %s", format)))
		return format
	}
}

func (s *scraper) translateTag(tag string) string {
	if tag == "Gang Giovanili" ||
		tag == "Raccolta" ||
		tag == "Tamarro" {
		return "" // ignore intentionally
	}

	switch tag {
	case "Arti Marziali":
		return "Martial Arts"
	case "Automobilismo":
		return "Racing"
	case "Avventura":
		return "Adventure"
	case "Azione":
		return "Action"
	case "Bambini":
		return "Children"
	case "Calcio":
		return "Soccer"
	case "Combattimento":
		return "Fighting"
	case "Commedia":
		return "Comedy"
	case "Crimine":
		return "Crime"
	case "Cucina":
		return "Cooking"
	case "Demenziale":
		return "Crazy"
	case "Demoni":
		return "Demons"
	case "Drammatico":
		return "Drama"
	case "Ecchi":
		return "Ecchi"
	case "Erotico":
		return "Erotic"
	case "Fantascienza":
		return "Science Fiction"
	case "Fantastico":
		return "Fantasy"
	case "Fantasy":
		return "Fantasy"
	case "Fiaba":
		return "Fairy Tale"
	case "Furry":
		return "Furry"
	case "Gang Giovanili":
		return "Youth Gang"
	case "Gender Bender":
		return "Gender Bender"
	case "Giallo":
		return "Mystery" // special kind of Italian murder mystery
	case "Gioco":
		return "Game"
	case "Guerra":
		return "War"
	case "Harem":
		return "Harem"
	case "Hentai":
		return "Hentai"
	case "Horror":
		return "Horror"
	case "Lolicon":
		return "Lolicon"
	case "Magia":
		return "Magic"
	case "Majokko":
		return "Magical Girl"
	case "Mecha":
		return "Mecha"
	case "Mistero":
		return "Mystery"
	case "Musica":
		return "Music"
	case "Parodia":
		return "Parody"
	case "Politica":
		return "Politics"
	case "Poliziesco":
		return "Police"
	case "Psicologico":
		return "Psychological"
	case "Pubblico Adulto":
		return "Adult"
	case "Pubblico Maturo":
		return "Mature"
	case "Reverse-harem":
		return "Reverse-harem"
	case "Scolastico":
		return "School"
	case "Sentimentale":
		return "Sentimental"
	case "Shotacon":
		return "Shotacon"
	case "Shoujo-Ai":
		return "Shoujo-Ai"
	case "Shounen-Ai":
		return "Shounen-Ai"
	case "Slice of Life":
		return "Slice of Life"
	case "Smut":
		return "Smut"
	case "Soprannaturale":
		return "Supernatural"
	case "Sperimentale":
		return "Experimental"
	case "Splatter":
		return "Violent"
	case "Sport":
		return "Sports"
	case "Storico":
		return "Historical"
	case "Supereroi":
		return "Super Hero"
	case "Superpoteri":
		return "Super Powers"
	case "Thriller":
		return "Thriller"
	case "Visual novel":
		return "Visual Novel"
	case "Yaoi":
		return "Yaoi"
	case "Yuri":
		return "Yuri"
	default:
		s.cxt.Error("main", errors.New(fmt.Sprintf("Unknown tag: %s", tag)))
		return ""
	}
}

func (s *scraper) commitChanges() error {
	db := s.as.db.DB(MongoDbName)
	shows := db.C(MongoShowsCollectionName)
	shows.DropCollection()

	iter := s.shows.Find(nil).Iter()
	result := Show{}
	for iter.Next(&result) {
		if len(result.ScreenshotUrls) > 0 {
			err := shows.Insert(&result)
			if err != nil {
				return err
			}
		}
	}

	s.as.db.DB(MongoTempDbName).DropDatabase()
	return nil
}

func (s *scraper) downloadPage(page string) string {
	return s.proxyTarget.Get(page, func(s string) error {
		if !strings.Contains(s, "Informazione su anime, manga e fansub") {
			return errors.New("looks like the site must have been blocked")
		}
		return nil
	})
}

func titleCase(str string) string {
	ret := ""
	for _, word := range strings.Split(str, " ") {
		ret += strings.ToUpper(word[:1]) + word[1:] + " "
	}
	return strings.TrimSpace(ret)
}
