package main

import (
	"fmt"
	"github.com/garbotron/goshots/core"
	"github.com/garbotron/goshots/providers/animeclips"
	"github.com/garbotron/goshots/providers/animeshots"
	"github.com/garbotron/goshots/providers/gamershots"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"time"
)

const httpPort = 80

func main() {

	rand.Seed(time.Now().UTC().UnixNano())

	providers := []goshots.Provider{
		&gamershots.Gamershots{},
		&animeshots.Animeshots{},
		&animeclips.Animeclips{},
	}

	if err := goshots.ServerInit("/go", providers...); err != nil {
		log.Fatal(err)
	} else {
		http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil)
	}
}
