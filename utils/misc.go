package utils

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"
)

func CreateCautiousClient(proxy func(*http.Request) (*url.URL, error)) *http.Client {
	client := *http.DefaultClient
	client.Transport = &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, time.Second*5)
		},
		Proxy: proxy,
		ResponseHeaderTimeout: time.Second * 5,
		DisableKeepAlives:     true,
	}
	client.Timeout = time.Second * 10
	return &client
}

func DownloadPage(url string) (string, error) {
	resp, err := CreateCautiousClient(nil).Get(url)
	if err != nil {
		return "", err
	}
	return RespToString(resp, err)
}

func RespToString(resp *http.Response, err error) (string, error) {
	if err != nil {
		return "", err
	} else {
		defer resp.Body.Close()
		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(contents), nil
	}
}
