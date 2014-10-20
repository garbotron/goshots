package gamershots

import (
	"fmt"
	"github.com/garbotron/goshots/core"
	"gopkg.in/mgo.v2/bson"
	"time"
)

type FilterListType int

const Whitelist FilterListType = 0
const Blacklist FilterListType = 1

func (lt *FilterListType) name() string {
	if *lt == Whitelist {
		return "Whitelist"
	} else {
		return "Blacklist"
	}
}

type GamershotsFilterContext struct {
	gs                *Gamershots
	includeRereleases bool
}

type GamershotsFilter interface {
	goshots.Filter
	Config(cxt *GamershotsFilterContext, vals []int)
	Apply(cxt *GamershotsFilterContext, vals []int) []bson.M
}

func GamershotsFilters() []GamershotsFilter {
	return []GamershotsFilter{
		gsFilterSystem{},
		gsFilterIncludeRereleases{},
		gsFilterGenre{Whitelist},
		gsFilterGenre{Blacklist},
		gsFilterTheme{Whitelist},
		gsFilterTheme{Blacklist},
		gsFilterNumReviews{},
		gsFilterAverageReviewScore{},
		gsFilterReleaseDate{},
		gsFilterReleaseRegion{}}
}

func (_ Gamershots) Filters() []goshots.Filter {
	filters := GamershotsFilters()
	ret := make([]goshots.Filter, len(filters))
	for i := 0; i < len(filters); i++ {
		ret[i] = filters[i]
	}
	return ret
}

//----------------------------------------------------------------------------//

type gsFilterSystem struct{}

func (_ gsFilterSystem) Name() string {
	return "Filter by System"
}

func (_ gsFilterSystem) Prompt() string {
	return ""
}

func (_ gsFilterSystem) Type() goshots.FilterType {
	return goshots.FilterTypeSelectMany
}

func (_ gsFilterSystem) Names(p goshots.Provider) ([]string, error) {
	gs := p.(*Gamershots)
	return gs.GetAllSystems(), nil
}

func (_ gsFilterSystem) DefaultValues() []int {
	return []int{}
}

func (_ gsFilterSystem) Config(cxt *GamershotsFilterContext, vals []int) {
}

func (_ gsFilterSystem) Apply(cxt *GamershotsFilterContext, vals []int) []bson.M {
	systems := cxt.gs.GetAllSystems()
	sel := []string{}
	for _, i := range vals {
		if i < len(systems) {
			sel = append(sel, systems[i])
		}
	}
	if cxt.includeRereleases {
		return []bson.M{
			bson.M{"$or": []bson.M{
				bson.M{"primarysystems": bson.M{"$in": sel}},
				bson.M{"rereleasesystems": bson.M{"$in": sel}},
			}},
		}
	} else {
		return []bson.M{
			bson.M{"primarysystems": bson.M{"$in": sel}},
		}
	}
}

//----------------------------------------------------------------------------//

type gsFilterIncludeRereleases struct{}

func (_ gsFilterIncludeRereleases) Name() string {
	return "Include Re-releases?"
}

func (_ gsFilterIncludeRereleases) Prompt() string {
	return ""
}

func (_ gsFilterIncludeRereleases) Type() goshots.FilterType {
	return goshots.FilterTypeSelectOne
}

func (_ gsFilterIncludeRereleases) Names(_ goshots.Provider) ([]string, error) {
	return []string{"Primary Systems Only", "Include Re-releases"}, nil
}

func (_ gsFilterIncludeRereleases) DefaultValues() []int {
	return []int{0}
}

func (_ gsFilterIncludeRereleases) Config(cxt *GamershotsFilterContext, vals []int) {
	cxt.includeRereleases = len(vals) > 0 && vals[0] == 1
}

func (_ gsFilterIncludeRereleases) Apply(cxt *GamershotsFilterContext, vals []int) []bson.M {
	return []bson.M{}
}

//----------------------------------------------------------------------------//

type gsFilterGenre struct {
	listType FilterListType
}

func (f gsFilterGenre) Name() string {
	return fmt.Sprintf("Filter by Genre: %s", f.listType.name())
}

func (_ gsFilterGenre) Prompt() string {
	return ""
}

func (_ gsFilterGenre) Type() goshots.FilterType {
	return goshots.FilterTypeSelectMany
}

func (_ gsFilterGenre) Names(p goshots.Provider) ([]string, error) {
	gs := p.(*Gamershots)
	return gs.GetAllGenres(), nil
}

func (_ gsFilterGenre) DefaultValues() []int {
	return []int{}
}

func (_ gsFilterGenre) Config(cxt *GamershotsFilterContext, vals []int) {
}

func (f gsFilterGenre) Apply(cxt *GamershotsFilterContext, vals []int) []bson.M {
	genres := cxt.gs.GetAllGenres()
	sel := []string{}
	for _, i := range vals {
		if i < len(genres) {
			sel = append(sel, genres[i])
		}
	}
	if f.listType == Blacklist {
		return []bson.M{
			bson.M{"genres.0": bson.M{"$exists": true}},
			bson.M{"genres": bson.M{"$not": bson.M{"$in": sel}}},
		}
	} else {
		return []bson.M{
			bson.M{"genres": bson.M{"$in": sel}},
		}
	}
}

//----------------------------------------------------------------------------//

type gsFilterTheme struct {
	listType FilterListType
}

func (f gsFilterTheme) Name() string {
	return fmt.Sprintf("Filter by Theme: %s", f.listType.name())
}

func (_ gsFilterTheme) Prompt() string {
	return ""
}

func (_ gsFilterTheme) Type() goshots.FilterType {
	return goshots.FilterTypeSelectMany
}

func (_ gsFilterTheme) Names(p goshots.Provider) ([]string, error) {
	gs := p.(*Gamershots)
	return gs.GetAllThemes(), nil
}

func (_ gsFilterTheme) DefaultValues() []int {
	return []int{}
}

func (_ gsFilterTheme) Config(cxt *GamershotsFilterContext, vals []int) {
}

func (f gsFilterTheme) Apply(cxt *GamershotsFilterContext, vals []int) []bson.M {
	themes := cxt.gs.GetAllThemes()
	sel := []string{}
	for _, i := range vals {
		if i < len(themes) {
			sel = append(sel, themes[i])
		}
	}
	if f.listType == Blacklist {
		return []bson.M{
			bson.M{"themes.0": bson.M{"$exists": true}},
			bson.M{"themes": bson.M{"$not": bson.M{"$in": sel}}},
		}
	} else {
		return []bson.M{
			bson.M{"themes": bson.M{"$in": sel}},
		}
	}
}

//----------------------------------------------------------------------------//

type gsFilterReleaseDate struct{}

func (_ gsFilterReleaseDate) Name() string {
	return "Filter by Release Date"
}

func (_ gsFilterReleaseDate) Prompt() string {
	return "Original release date"
}

func (_ gsFilterReleaseDate) Type() goshots.FilterType {
	return goshots.FilterTypeNumberRange
}

func (_ gsFilterReleaseDate) Names(_ goshots.Provider) ([]string, error) {
	return nil, nil
}

func (_ gsFilterReleaseDate) DefaultValues() []int {
	return []int{1950, time.Now().Year()}
}

func (_ gsFilterReleaseDate) Config(cxt *GamershotsFilterContext, vals []int) {
}

func (_ gsFilterReleaseDate) Apply(cxt *GamershotsFilterContext, vals []int) []bson.M {
	if len(vals) < 2 {
		return []bson.M{}
	}
	return []bson.M{
		bson.M{"releasedate": bson.M{"$gte": vals[0]}},
		bson.M{"releasedate": bson.M{"$lte": vals[1]}},
	}
}

//----------------------------------------------------------------------------//

type gsFilterNumReviews struct{}

func (_ gsFilterNumReviews) Name() string {
	return "Filter by Number of Reviews"
}

func (_ gsFilterNumReviews) Prompt() string {
	return "Minimum number of press reviews"
}

func (_ gsFilterNumReviews) Type() goshots.FilterType {
	return goshots.FilterTypeNumber
}

func (_ gsFilterNumReviews) Names(_ goshots.Provider) ([]string, error) {
	return nil, nil
}

func (_ gsFilterNumReviews) DefaultValues() []int {
	return []int{3}
}

func (_ gsFilterNumReviews) Config(cxt *GamershotsFilterContext, vals []int) {
}

func (_ gsFilterNumReviews) Apply(cxt *GamershotsFilterContext, vals []int) []bson.M {
	if len(vals) < 1 {
		return []bson.M{}
	}
	return []bson.M{
		bson.M{"numreviews": bson.M{"$gte": vals[0]}},
	}
}

//----------------------------------------------------------------------------//

type gsFilterAverageReviewScore struct{}

func (_ gsFilterAverageReviewScore) Name() string {
	return "Filter by Review Score (Out of 100)"
}

func (_ gsFilterAverageReviewScore) Prompt() string {
	return "Average review score"
}

func (_ gsFilterAverageReviewScore) Type() goshots.FilterType {
	return goshots.FilterTypeNumberRange
}

func (_ gsFilterAverageReviewScore) Names(_ goshots.Provider) ([]string, error) {
	return nil, nil
}

func (_ gsFilterAverageReviewScore) DefaultValues() []int {
	return []int{0, 100}
}

func (_ gsFilterAverageReviewScore) Config(cxt *GamershotsFilterContext, vals []int) {
}

func (_ gsFilterAverageReviewScore) Apply(cxt *GamershotsFilterContext, vals []int) []bson.M {
	if len(vals) < 1 {
		return []bson.M{}
	}
	return []bson.M{
		bson.M{"averagereviewscore": bson.M{"$gte": vals[0]}},
	}
}

//----------------------------------------------------------------------------//

type gsFilterReleaseRegion struct{}

func (_ gsFilterReleaseRegion) Name() string {
	return "Filter by Release Region"
}
func (_ gsFilterReleaseRegion) Prompt() string {
	return ""
}

func (_ gsFilterReleaseRegion) Type() goshots.FilterType {
	return goshots.FilterTypeSelectOne
}

func (_ gsFilterReleaseRegion) Names(_ goshots.Provider) ([]string, error) {
	return []string{"All Releases", "US Releases Only"}, nil
}

func (_ gsFilterReleaseRegion) DefaultValues() []int {
	return []int{0}
}

func (_ gsFilterReleaseRegion) Config(cxt *GamershotsFilterContext, vals []int) {
}

func (_ gsFilterReleaseRegion) Apply(cxt *GamershotsFilterContext, vals []int) []bson.M {
	if len(vals) > 0 && vals[0] == 1 {
		return []bson.M{
			bson.M{"regions": bson.M{"$in": []string{"United States"}}},
		}
	} else {
		return []bson.M{}
	}
}
