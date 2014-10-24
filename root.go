package goshots

import (
	"github.com/garbotron/goshots/core"
	"github.com/garbotron/goshots/providers/animeclips"
	"github.com/garbotron/goshots/providers/animeshots"
	"github.com/garbotron/goshots/providers/gamershots"
	"math/rand"
	"time"
)

func Init() error {
	rand.Seed(time.Now().UTC().UnixNano())

	providers := []goshots.Provider{
		&gamershots.Gamershots{},
		&animeshots.Animeshots{},
		&animeclips.Animeclips{},
	}

	return goshots.ServerInit("/go", providers...)
}
