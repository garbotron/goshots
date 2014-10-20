package utils

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ProxyTarget struct {
	numConcurrentProxies int
	curProxyListPage     int
	requests             chan *renderRequest
	proxies              chan *proxy
	closing              chan struct{}
	closed               chan struct{}
}

func CreateProxyTarget(numConcurrentProxies int) *ProxyTarget {
	t := ProxyTarget{}
	t.numConcurrentProxies = numConcurrentProxies
	t.proxies = make(chan *proxy, 1)
	t.requests = make(chan *renderRequest, numConcurrentProxies)
	t.closing = make(chan struct{})
	t.closed = make(chan struct{})

	go t.runProxyProducer()
	for i := 0; i < numConcurrentProxies; i++ {
		go t.runRequestRenderer()
	}
	return &t
}

func (t *ProxyTarget) Get(page string, validator func(string) error) string {
	req := renderRequest{page, validator, make(chan string)}
	t.requests <- &req
	return <-req.result
}

func (t *ProxyTarget) Dispose() {
	proxyLog("[PROXY] shutting down...")
	t.closing <- struct{}{}
	for i := 0; i < t.numConcurrentProxies; i++ {
		<-t.closed
	}
	proxyLog("[PROXY] shutdown complete!")
}

const maxRendersPerProxy = 500
const msDelayBetweenRequests = 1
const enableProxyLogging = false

var proxyListPages = []string{"http://www.us-proxy.org/", "http://free-proxy-list.net/anonymous-proxy.html"}

type renderRequest struct {
	page      string
	validator func(string) error
	result    chan string
}

type proxy struct {
	address string
	port    int
}

func (p *proxy) name() string {
	return fmt.Sprintf("%s:%d", p.address, p.port)
}

func (p *proxy) url() *url.URL {
	u, _ := url.Parse(fmt.Sprintf("http://%s", p.name()))
	return u
}

func proxyLog(format string, args ...interface{}) {
	if enableProxyLogging {
		fmt.Println(fmt.Sprintf(format, args...))
	}
}

func (t *ProxyTarget) downloadWithProxy(page string, proxy *proxy, validator func(string) error) (string, error) {
	client := *http.DefaultClient
	client.Transport = &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, time.Second*5)
		},
		Proxy: func(r *http.Request) (*url.URL, error) {
			return proxy.url(), nil
		},
		ResponseHeaderTimeout: time.Second * 5,
	}
	client.Timeout = time.Second * 10

	str, err := RespToString(client.Get(page))
	if err != nil {
		return "", err
	}

	if strings.Contains(str, "<title>Access Denined</title>") {
		// this is a web proxy that ran into a user limit
		return "", errors.New("web proxy user limit exceeded")
	}

	if validator != nil {
		if err := validator(str); err != nil {
			return "", err
		}
	}
	return str, nil
}

func (t *ProxyTarget) downloadWithoutProxy(page string) (string, error) {
	return RespToString(http.Get(page))
}

func (t *ProxyTarget) downloadProxyListRootPage() *goquery.Document {
	html, err := t.downloadWithoutProxy(proxyListPages[t.curProxyListPage])
	if err != nil {
		proxyLog("[PROXY] failed root page download, retrying...")
		<-time.After(time.Second * 5)
		return t.downloadProxyListRootPage()
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		proxyLog("[PROXY] failed root page parse, retrying...")
		<-time.After(time.Second * 5)
		return t.downloadProxyListRootPage()
	}

	t.curProxyListPage++
	if t.curProxyListPage >= len(proxyListPages) {
		t.curProxyListPage = 0
	}
	return doc
}

func (t *ProxyTarget) testProxy(proxy *proxy) error {
	str, err := t.downloadWithProxy("http://checkip.dyndns.org/", proxy, nil)
	if err != nil {
		return err
	}
	if !strings.Contains(str, proxy.address) {
		return errors.New("proxy didn't do its job")
	}
	return nil
}

func (t *ProxyTarget) runProxyProducer() {
	running := true
	for running {
		doc := t.downloadProxyListRootPage()
		proxyLog("[PROXY] root proxy list loaded!")
		doc.Find("#proxylisttable tr").EachWithBreak(func(_ int, s *goquery.Selection) bool {

			cells := s.Find("td")
			if cells.Length() < 4 {
				return true
			}

			anonymity := cells.Eq(4).Text()
			if anonymity != "elite proxy" && anonymity != "anonymous" {
				return true
			}
			ip := cells.Eq(0).Text()
			port, err := strconv.Atoi(cells.Eq(1).Text())
			if err != nil {
				return true
			}

			proxy := proxy{ip, port}
			if err := t.testProxy(&proxy); err != nil {
				proxyLog("[PROXY %s] failed: %s, skipping...", proxy.name(), err.Error())
				return true
			}

			select {
			case t.proxies <- &proxy:
				proxyLog("[PROXY %s] started!", proxy.name())
				return true
			case <-t.closing:
				proxyLog("[PROXY] producer shut down!")
				running = false
				return false
			}
		})
	}

	close(t.proxies)
	close(t.requests)
}

func (t *ProxyTarget) renderRequestsUntilFail() (shouldStop bool) {
	var proxy *proxy
	var req *renderRequest
	var ok bool

	if proxy, ok = <-t.proxies; !ok {
		return true
	}

	for i := 0; i < maxRendersPerProxy; i++ {
		if req, ok = <-t.requests; !ok {
			return true
		}

		str, err := t.downloadWithProxy(req.page, proxy, req.validator)
		for err != nil {
			proxyLog("[PROXY %s] render failed (%s), switching...", proxy.name(), err.Error())
			t.requests <- req // re-enqueue the request
			return false
		}

		req.result <- str
		<-time.After(time.Millisecond * msDelayBetweenRequests)
	}

	proxyLog("[PROXY %s] render limit exceeded, switching...", proxy.name())
	return false
}

func (t *ProxyTarget) runRequestRenderer() {

	shouldStop := false
	for !shouldStop {
		shouldStop = t.renderRequestsUntilFail()
	}

	proxyLog("[PROXY] renderer shut down!")
	t.closed <- struct{}{}
}
