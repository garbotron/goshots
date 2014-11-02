package goshots

import (
	"github.com/garbotron/goshots/core"
	"github.com/garbotron/goshots/providers/animeclips"
	"github.com/garbotron/goshots/providers/animeshots"
	"github.com/garbotron/goshots/providers/gamershots"
	"github.com/gorilla/mux"
	"math/rand"
	"time"
)

func Init(r *mux.Router) error {
	rand.Seed(time.Now().UTC().UnixNano())

	providers := []goshots.Provider{
		&gamershots.Gamershots{},
		&animeshots.Animeshots{},
		&animeclips.Animeclips{},
	}

	return goshots.ServerInit(r, providers...)
}
