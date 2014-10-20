package gamershots

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/garbotron/goshots/core"
	"github.com/garbotron/goshots/utils"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const numGameScrapers = 30
const maxScreenshotsPerGame = 50
const absFirstYear = 1972

var absLastYear = time.Now().Year()

func (gs *Gamershots) CanScrape() bool {
	return true
}

func (gs *Gamershots) StartScraping(context goshots.ScraperContext) goshots.Scraper {

	db := gs.db.DB(MongoTempDbName)
	db.DropDatabase() // in case there was any stale data left over
	games := db.C(MongoGamesCollectionName)
	listings := db.C("listings")

	s := scraper{
		cxt:         context,
		gs:          gs,
		games:       games,
		listings:    listings,
		proxyTarget: utils.CreateProxyTarget(20),
		stage:       "beginning scan",
		abortSignal: make(chan struct{}, 1),
		testMode:    false,
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
	cur, _ = s.games.Count()
	total, _ = s.listings.Count()
	return
}

type scraper struct {
	cxt         goshots.ScraperContext
	gs          *Gamershots
	listings    *mgo.Collection // basic game listing data, per the "gameListing" struct
	games       *mgo.Collection // full game data used for final result
	proxyTarget *utils.ProxyTarget
	stage       string
	testMode    bool // debug feature: parse vastly less input
	aborting    bool
	abortSignal chan struct{}
}

type gameListing struct {
	Year      int
	ShortName string
	LongName  string
}

func (s *scraper) scrape() {

	// we scrape in 2 phases:
	// 1 - look at the listing pages only - collect game year/short name/long name
	// 2 - for each game, scrape all the related pages

	s.cxt.Log("collecting game listings...")
	s.stage = "collecting game list"
	if err := s.scrapeGameListings(); err != nil {
		s.finish(err)
		return
	}

	s.cxt.Log("starting full scan...")
	s.stage = "scanning"
	if err := s.scrapeGames(); err != nil {
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

func readManyErrors(done <-chan error, count int) error {
	var ret error = nil
	for i := 0; i < count; i++ {
		if err := <-done; err != nil {
			ret = err
		}
	}
	return ret
}

func (s *scraper) scrapeGameListings() error {
	done := make(chan error)
	firstYear := absFirstYear
	lastYear := absLastYear
	if s.testMode {
		firstYear = absFirstYear + ((absLastYear - absFirstYear) / 2)
		lastYear = firstYear + 1
	}
	for year := firstYear; year <= lastYear; year++ {
		go s.scrapeGameListingPage(year, 0, done)
	}
	return readManyErrors(done, (lastYear-firstYear)+1)
}

func (s *scraper) scrapeGameListingPage(year int, offset int, done chan<- error) {

	numGames := int32(0)
	url := fmt.Sprintf("http://www.mobygames.com/browse/games/%d/offset,%d/list-games/", year, offset)
	doc := s.downloadPage(url)
	doc.Find("#mof_object_list a").Each(func(_ int, sel *goquery.Selection) {

		attr, ok := sel.Attr("href")
		if ok && !strings.HasPrefix(attr, "/game/") {
			return
		}
		short := attr[len("/game/"):]
		long := strings.TrimSpace(sel.Text())

		_, err := s.listings.Upsert(bson.M{"shortname": short}, &gameListing{year, short, long})
		if err != nil {
			s.cxt.Error("listings", err)
		}
		s.cxt.Log("found '%s' (%d)", long, year)
		numGames++
	})

	if s.aborting {
		done <- goshots.ScraperAbortError()
		return
	}

	if !s.testMode && numGames == 25 {
		// we did an entire page full - load the next one
		s.scrapeGameListingPage(year, offset+1, done)
	} else {
		done <- nil
	}
}

func (s *scraper) scrapeGames() error {

	listings := make(chan *gameListing)
	done := make(chan struct{})
	for i := 0; i < numGameScrapers; i++ {
		go s.scrapeGamesFromChannel(listings, done)
	}

	listing := gameListing{}
	iter := s.listings.Find(nil).Iter()
	for iter.Next(&listing) {
		if s.aborting {
			iter.Close()
			break
		}
		copy := listing
		select {
		case listings <- &copy:
		case <-s.abortSignal:
		}
		if s.aborting {
			iter.Close()
			break
		}
	}
	close(listings)
	for i := 0; i < numGameScrapers; i++ {
		<-done
	}

	if s.aborting {
		return goshots.ScraperAbortError()
	} else {
		return nil
	}
}

func (s *scraper) scrapeGamesFromChannel(listings <-chan *gameListing, done chan<- struct{}) {
	for game := range listings {
		if !s.scrapeGame(game) {
			break // aborted
		}
	}
	done <- struct{}{}
}

func (s *scraper) scrapeGame(listing *gameListing) bool {

	if s.aborting {
		return false
	}

	s.cxt.Log("starting game: %s (%d)...", listing.LongName, listing.Year)

	docMain := s.downloadPage(fmt.Sprintf("http://www.mobygames.com/game/%s/", listing.ShortName))
	docRank := s.downloadPage(fmt.Sprintf("http://www.mobygames.com/game/%s/mobyrank", listing.ShortName))
	docReleases := s.downloadPage(fmt.Sprintf("http://www.mobygames.com/game/%s/release-info", listing.ShortName))
	docScreenshots := s.downloadPage(fmt.Sprintf("http://www.mobygames.com/game/%s/screenshots", listing.ShortName))

	genres, themes := s.scrapeGameMain(docMain)
	numReviews, avgReviewScore := s.scrapeGameRank(docRank)
	countries, primaryReleases, rereleases := s.scrapeGameReleases(docReleases, listing.Year)
	screenshots := s.scrapeGameScreenshots(docScreenshots)

	genres = removeDuplicates(genres)
	themes = removeDuplicates(themes)
	countries = removeDuplicates(countries)
	primaryReleases = removeDuplicates(primaryReleases)
	rereleases = removeDuplicates(rereleases)
	screenshots = removeDuplicates(screenshots)

	if s.aborting {
		return false
	}

	// Apply all changes to the database
	game := Game{
		Name:               listing.LongName,
		ReleaseDate:        listing.Year,
		NumReviews:         numReviews,
		AverageReviewScore: avgReviewScore,
		ScreenshotUrls:     screenshots,
		PrimarySystems:     primaryReleases,
		RereleaseSystems:   rereleases,
		Genres:             genres,
		Themes:             themes,
		Regions:            countries,
	}
	s.games.Insert(&game)
	s.cxt.Log("completed game: %s (%d screenshots)", listing.LongName, len(screenshots))

	return true
}

func (s *scraper) scrapeGameMain(doc *goquery.Document) (genres []string, themes []string) {

	genres = []string{}
	themes = []string{}

	doc.Find("#coreGameGenre div").Each(func(_ int, div *goquery.Selection) {
		if div.Text() == "Genre" || div.Text() == "Genres" {
			div.Next().Find("a").Each(func(_ int, a *goquery.Selection) {
				genres = append(genres, a.Text())
			})
		}
		if div.Text() == "Theme" || div.Text() == "Themes" {
			div.Next().Find("a").Each(func(_ int, a *goquery.Selection) {
				themes = append(themes, a.Text())
			})
		}
	})

	return
}

func (s *scraper) scrapeGameRank(doc *goquery.Document) (numReviews int, avgReviewScore int) {
	reviewTotal := 0
	numReviews = 0
	doc.Find("div.fl.scoreBoxMed").Each(func(_ int, div *goquery.Selection) {
		numReviews++
		x, err := strconv.Atoi(div.Text())
		if err != nil {
			s.cxt.Error(doc.Url.String(), err)
		}
		reviewTotal += x
	})
	if numReviews == 0 {
		avgReviewScore = 0
	} else {
		avgReviewScore = reviewTotal / numReviews
	}
	return
}

func (s *scraper) scrapeGameReleases(doc *goquery.Document, releaseDate int) (countries []string, primaryReleases []string, rereleases []string) {

	type systemYear struct {
		system string
		year   int
	}

	countries = []string{}
	systemYears := []systemYear{}

	countryTitles := doc.Find("div.relInfoTitle:contains(\"Countr\")")
	countryTitles.Each(func(_ int, countryTitle *goquery.Selection) {

		indivCountries := []string{}

		countryHolder := countryTitle.Parent()
		countrySpans := countryHolder.Find("div.relInfoDetails span")
		countrySpans.Each(func(_ int, countrySpan *goquery.Selection) {
			newCountry := strings.Trim(countrySpan.Text(), ", ")
			if newCountry == "" {
				return
			}
			for _, country := range indivCountries {
				if country == newCountry {
					return
				}
			}
			indivCountries = append(indivCountries, newCountry)
		})

		if len(indivCountries) == 0 {
			s.cxt.Error(doc.Url.String(), errors.New("could not find any countries inside country div"))
			return
		}

		for _, country := range indivCountries {
			countries = append(countries, country)
		}

		system := countryHolder.Parent().PrevAllFiltered("h2").First().Text()
		if system == "" {
			s.cxt.Error(doc.Url.String(), errors.New("could not find system h2"))
			return
		}

		for _, systemYear := range systemYears {
			if systemYear.system == system {
				return
			}
		}

		dateHolder := countryHolder.Next()
		dateDiv := dateHolder.Find("div.relInfoDetails")
		date := dateDiv.Text()
		commaIdx := strings.Index(date, ",")
		if commaIdx != -1 {
			date = strings.TrimSpace(date[commaIdx+1:])
		}
		year, err := strconv.Atoi(date)
		if err != nil {
			s.cxt.Error(doc.Url.String(), err)
			return
		}
		systemYears = append(systemYears, systemYear{system, year})
	})

	primaryReleases = []string{}
	rereleases = []string{}

	for _, systemYear := range systemYears {
		if systemYear.year-releaseDate <= 2 {
			// the game was released in the same year or just 1/2 years apart
			primaryReleases = append(primaryReleases, systemYear.system)
		} else {
			rereleases = append(rereleases, systemYear.system)
		}
	}
	return
}

func (s *scraper) scrapeGameScreenshots(doc *goquery.Document) []string {

	ssUrls := []string{}

	doc.Find("div.mobythumbnail").Each(func(_ int, outer *goquery.Selection) {

		if s.aborting {
			return
		}

		allTxt := strings.ToLower(outer.Text())
		if strings.Contains(allTxt, "title") || strings.Contains(allTxt, "main menu") {
			return
		}

		href, ok := outer.Find("img").Parent().Attr("href")
		if !ok || !strings.HasPrefix(href, "/game/") || !strings.Contains(href, "/screenshots/") {
			return
		}

		linkDoc := s.downloadPage("http://www.mobygames.com" + href)
		linkDoc.Find("img").Each(func(_ int, img *goquery.Selection) {
			src, ok := img.Attr("src")

			if !ok || !strings.HasPrefix(src, "/images/shots/") {
				return
			}

			ssUrl := "http://www.mobygames.com" + src
			if len(ssUrl) > 255 {
				return // won't fit in the DB table
			}

			ssUrls = append(ssUrls, ssUrl)
		})
	})

	screenshots := []string{}
	for len(ssUrls) > 0 && len(screenshots) < maxScreenshotsPerGame {
		idx := rand.Int() % len(ssUrls)
		screenshots = append(screenshots, ssUrls[idx])
		ssUrls = append(ssUrls[:idx], ssUrls[idx+1:]...) // delete idx from slice
	}
	return screenshots
}

func (s *scraper) commitChanges() error {

	db := s.gs.db.DB(MongoDbName)
	games := db.C(MongoGamesCollectionName)
	games.DropCollection()

	iter := s.games.Find(nil).Iter()
	result := Game{}
	for iter.Next(&result) {
		if len(result.ScreenshotUrls) > 0 {
			err := games.Insert(&result)
			if err != nil {
				return err
			}
		}
	}

	s.gs.db.DB(MongoTempDbName).DropDatabase()
	return nil
}

func (s *scraper) downloadPage(page string) *goquery.Document {
	html := s.proxyTarget.Get(page, func(s string) error {
		if !strings.Contains(s, "/images/mobygames-logo-sm.png") {
			return errors.New("couldn't find logo")
		}
		return nil
	})
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		s.cxt.Error(page, err)
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(""))
		return doc
	}
	return doc
}

func removeDuplicates(lst []string) []string {
	for i := 0; i < len(lst)-1; i++ {
		for j := i + 1; j < len(lst); j++ {
			if lst[i] == lst[j] {
				lst = append(lst[:j], lst[j+1:]...) // delete idx from slice
				j--
			}
		}
	}
	return lst
}
