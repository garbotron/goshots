package utils

import (
	"io/ioutil"
	"net/http"
)

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
