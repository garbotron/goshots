package animeshots

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

type AnimeshotsFilter interface {
	goshots.Filter
	Apply(as *Animeshots, vals []int) []bson.M
}

func AnimeshotsFilters() []AnimeshotsFilter {
	return []AnimeshotsFilter{
		asFilterTag{Whitelist},
		asFilterTag{Blacklist},
		asFilterType{},
		asFilterDate{}}
}

func (_ Animeshots) Filters() []goshots.Filter {
	filters := AnimeshotsFilters()
	ret := make([]goshots.Filter, len(filters))
	for i := 0; i < len(filters); i++ {
		ret[i] = filters[i]
	}
	return ret
}

//----------------------------------------------------------------------------//

type asFilterTag struct {
	listType FilterListType
}

func (f asFilterTag) Name() string {
	return fmt.Sprintf("Filter by Tag: %s", f.listType.name())
}

func (_ asFilterTag) Prompt() string {
	return ""
}

func (_ asFilterTag) Type() goshots.FilterType {
	return goshots.FilterTypeSelectMany
}

func (_ asFilterTag) Names(p goshots.Provider) ([]string, error) {
	as := p.(*Animeshots)
	return as.GetAllTags(), nil
}

func (_ asFilterTag) DefaultValues() []int {
	return []int{}
}

func (f asFilterTag) Apply(as *Animeshots, vals []int) []bson.M {
	tags := as.GetAllTags()
	sel := []string{}
	for _, i := range vals {
		if i < len(tags) {
			sel = append(sel, tags[i])
		}
	}
	if f.listType == Blacklist {
		return []bson.M{
			bson.M{"tags.0": bson.M{"$exists": true}},
			bson.M{"tags": bson.M{"$not": bson.M{"$in": sel}}},
		}
	} else {
		return []bson.M{
			bson.M{"tags": bson.M{"$in": sel}},
		}
	}
}

//----------------------------------------------------------------------------//

type asFilterType struct{}

func (_ asFilterType) Name() string {
	return "Filter by Type"
}

func (_ asFilterType) Prompt() string {
	return ""
}

func (_ asFilterType) Type() goshots.FilterType {
	return goshots.FilterTypeSelectMany
}

func (_ asFilterType) Names(p goshots.Provider) ([]string, error) {
	as := p.(*Animeshots)
	return as.GetAllTypes(), nil
}

func (_ asFilterType) DefaultValues() []int {
	return []int{}
}

func (_ asFilterType) Apply(as *Animeshots, vals []int) []bson.M {
	types := as.GetAllTypes()
	sel := []string{}
	for _, i := range vals {
		if i < len(types) {
			sel = append(sel, types[i])
		}
	}
	return []bson.M{
		bson.M{"type": bson.M{"$in": sel}},
	}
}

//----------------------------------------------------------------------------//

type asFilterDate struct{}

func (_ asFilterDate) Name() string {
	return "Filter by Date"
}

func (_ asFilterDate) Prompt() string {
	return "Original release date"
}

func (_ asFilterDate) Type() goshots.FilterType {
	return goshots.FilterTypeNumberRange
}

func (_ asFilterDate) Names(_ goshots.Provider) ([]string, error) {
	return nil, nil
}

func (_ asFilterDate) DefaultValues() []int {
	return []int{1900, time.Now().Year()}
}

func (_ asFilterDate) Apply(as *Animeshots, vals []int) []bson.M {
	if len(vals) < 2 {
		return []bson.M{}
	}
	return []bson.M{
		bson.M{"hasyear": true},
		bson.M{"year": bson.M{"$gte": vals[0]}},
		bson.M{"year": bson.M{"$lte": vals[1]}},
	}
}
