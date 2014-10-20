package utils

import (
	"bitbucket.org/kardianos/osext"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
)

func FileInRunningDir(fileName string) string {
	dir, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(dir, fileName)
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
