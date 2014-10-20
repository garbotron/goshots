package goshots

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"time"
)

const maxLogLines = 100
const maxErrorLines = 50

var scraperContexts = make(map[string]*scraperContext)
var authUserIp = ""

func RenderScrapePage(genericData *RendererData) error {
	data := ScrapeRendererData{RendererData: *genericData}

	if data.Request.RemoteAddr != authUserIp {
		op := data.Request.FormValue("pass")
		if op != os.Getenv("GOSHOTSPASS") {
			return RenderTemplate("scrape-auth.goshots", data.Writer, &data)
		}
		authUserIp = data.Request.RemoteAddr
	}

	cxt, exists := scraperContexts[data.Provider.ShortName()]
	logDir := path.Join(os.ExpandEnv("$GOPATH"), "logs", "goshots")
	if !exists {
		cxt = &scraperContext{
			scraping:      false,
			logFilePath:   path.Join(logDir, data.Provider.ShortName()+".status.log"),
			errorFilePath: path.Join(logDir, data.Provider.ShortName()+".error.log"),
		}
		scraperContexts[data.Provider.ShortName()] = cxt
	}

	if data.Provider.CanScrape() {
		op := data.Request.FormValue("op")
		switch op {
		case "start":
			if !cxt.scraping {
				cxt.startTime = time.Now()
				cxt.scraping = true
				os.Remove(cxt.logFilePath)   // ignore errors
				os.Remove(cxt.errorFilePath) // ignore errors
				cxt.scraper = data.Provider.StartScraping(cxt)
			}
		case "abort":
			if cxt.scraping && !cxt.aborting {
				cxt.aborting = true
				cxt.scraper.Abort()
			}
		}
	}

	if !data.Provider.CanScrape() {
		data.Status = "N/A"
		data.Scraping = false
	} else if !cxt.scraping {
		data.Status = "Not Scraping"
		data.Scraping = false
	} else {
		stage, cur, total := cxt.scraper.Progress()
		data.Status = "Scraping"
		data.Scraping = true
		data.Stage = stage
		if total <= 0 {
			data.ProgressPercent = 0
		} else {
			data.ProgressPercent = (cur * 100) / total
		}
	}

	data.LogLines = readLastLines(cxt.logFilePath, maxLogLines)
	data.ErrorLines = readLastLines(cxt.errorFilePath, maxErrorLines)

	return RenderTemplate("scrape.goshots", data.Writer, &data)
}

type ScrapeRendererData struct {
	RendererData
	Status          string
	Scraping        bool
	Stage           string
	ProgressPercent int
	LogLines        []string
	ErrorLines      []string
}

type scraperContext struct {
	scraping      bool
	aborting      bool
	startTime     time.Time
	scraper       Scraper
	logFilePath   string
	errorFilePath string
}

func (s *scraperContext) Log(format string, a ...interface{}) {
	f, err := os.OpenFile(s.logFilePath, os.O_APPEND, 0666)
	if err != nil {
		os.MkdirAll(path.Dir(s.logFilePath), 0666)
		f, err = os.Create(s.logFilePath)
	}
	if err != nil {
		return
	}
	defer f.Close()

	now := time.Now()
	ts := fmt.Sprintf("%d.%d.%d: ", now.Hour(), now.Minute(), now.Second())
	f.WriteString(fmt.Sprintf(ts+format+"\n", a...))
}

func (s *scraperContext) Error(context string, logError error) {
	f, err := os.OpenFile(s.errorFilePath, os.O_APPEND, 0666)
	if err != nil {
		os.MkdirAll(path.Dir(s.errorFilePath), 0666)
		f, err = os.Create(s.errorFilePath)
	}
	if err != nil {
		return
	}
	defer f.Close()

	now := time.Now()
	f.WriteString(fmt.Sprintf("%d.%d.%d: [%s] %s\n", now.Hour(), now.Minute(), now.Second(), context, logError.Error()))
}

func (s *scraperContext) Done(err error) {
	s.scraping = false
	s.aborting = false
	if err == nil {
		s.Log("completed without errors: %s", time.Now().Sub(s.startTime).String())
	} else {
		s.Log("completed with errors: %s", time.Now().Sub(s.startTime).String())
		s.Error("complete", err)
	}
}

func readLastLines(filePath string, numLines int) []string {
	file, err := os.Open(filePath)
	if err != nil {
		return []string{}
	}
	defer file.Close()

	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	ret := []string{}
	for i := 0; i < numLines; i++ {
		idx := (len(lines) - 1) - i
		if idx < 0 {
			break
		}
		ret = append(ret, lines[idx])
	}

	return ret
}
