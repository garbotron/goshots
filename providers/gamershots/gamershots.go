package gamershots

import (
	"fmt"
	"github.com/garbotron/goshots/core"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"math/rand"
	"strings"
)

const MongoServerAddress = "107.191.119.248"
const MongoDbName = "gamershots"
const MongoTempDbName = "gamershots_wip"
const MongoGamesCollectionName = "games"

type Gamershots struct {
	db *mgo.Session
}

type Game struct {
	Name               string
	ReleaseDate        int // year of the first release
	NumReviews         int
	AverageReviewScore int // out of 100
	ScreenshotUrls     []string
	PrimarySystems     []string
	RereleaseSystems   []string
	Genres             []string
	Themes             []string
	Regions            []string
}

func (_ *Gamershots) ShortName() string {
	return "gamershots"
}

func (_ *Gamershots) PrettyName() string {
	return "Gamershots"
}

func (_ *Gamershots) Description() []template.HTML {
	return []template.HTML{
		"Gamershots is a drinking game for gamers.",
		"Check out a screenshot, see if you can name the game!",
		"The concept is simple, but I couldn't find another site on the net that does it.",
		"<b>If you can't name the game, take a drink!</b>",
		"<b>If you can, everyone else takes a drink and you go again!</b>",
	}
}

func (_ *Gamershots) Title() string {
	return "Gamershots: Name That Screenshot!"
}

func (_ *Gamershots) Prompt() string {
	return "What game is this?"
}

func (gs *Gamershots) Load() error {
	if gs.db != nil {
		gs.db.Close()
	}
	var err error
	gs.db, err = mgo.Dial(MongoServerAddress)
	return err
}

func (gs *Gamershots) RandomElem(filterValues *goshots.FilterValues) (interface{}, error) {

	db := gs.db.DB(MongoDbName).C(MongoGamesCollectionName)

	filters := GamershotsFilters()
	cxt := GamershotsFilterContext{gs: gs}
	for i, fv := range *filterValues {
		if fv.Enabled {
			filters[i].Config(&cxt, fv.Values)
		}
	}
	exprs := []bson.M{}
	for i, fv := range *filterValues {
		if fv.Enabled {
			for _, expr := range filters[i].Apply(&cxt, fv.Values) {
				exprs = append(exprs, expr)
			}
		}
	}

	var findCondition interface{} = nil
	if len(exprs) > 0 {
		findCondition = bson.M{"$and": exprs}
	}

	objIds := []struct {
		ID bson.ObjectId "_id"
	}{}
	err := db.Find(findCondition).Select(bson.M{"_id": 1}).All(&objIds)
	if err != nil {
		return nil, err
	}

	if len(objIds) == 0 {
		return nil, goshots.ElemNotFoundError()
	}

	elemIdx := rand.Int() % len(objIds)
	return objIds[elemIdx].ID, nil
}

func (gs *Gamershots) ElemSolution(elem interface{}) (string, error) {

	id := elem.(bson.ObjectId)
	db := gs.db.DB(MongoDbName).C(MongoGamesCollectionName)
	game := Game{}
	err := db.FindId(id).One(&game)
	if err != nil {
		return "", err
	}

	systems := game.PrimarySystems
	if len(systems) > 4 {
		systems = []string{systems[0], systems[1], systems[2], "..."}
	}

	return fmt.Sprintf("%s (%s)", game.Name, strings.Join(systems, ", ")), nil
}

func (gs *Gamershots) RenderContentHtml(elem interface{}) (template.HTML, error) {

	id := elem.(bson.ObjectId)
	db := gs.db.DB(MongoDbName).C(MongoGamesCollectionName)
	game := Game{}
	err := db.FindId(id).One(&game)
	if err != nil {
		return "", err
	}

	elemIdx := rand.Int() % len(game.ScreenshotUrls)

	str := "<div style=\"width:100%;height:100%;display:table;background-image:url(" +
		game.ScreenshotUrls[elemIdx] +
		");background-repeat:no-repeat;background-size:contain;background-position:center;-webkit-background-size:contain;-moz-background-size:contain;-o-background-size:contain;background-size:contain;\"> </div>"
	return template.HTML(str), nil
}

func (gs *Gamershots) GetAllSystems() []string {
	result := []struct {
		Name string "_id"
	}{}
	gs.db.DB(MongoDbName).C(MongoGamesCollectionName).Pipe(
		[]bson.M{
			{"$unwind": "$primarysystems"},
			{"$group": bson.M{"_id": "$primarysystems"}},
			{"$sort": bson.M{"_id": 1}}}).All(&result)

	ret := make([]string, len(result))
	for i := 0; i < len(result); i++ {
		ret[i] = result[i].Name
	}

	return ret
}

func (gs *Gamershots) GetAllGenres() []string {
	result := []struct {
		Name string "_id"
	}{}
	gs.db.DB(MongoDbName).C(MongoGamesCollectionName).Pipe(
		[]bson.M{
			{"$unwind": "$genres"},
			{"$group": bson.M{"_id": "$genres"}},
			{"$sort": bson.M{"_id": 1}}}).All(&result)

	ret := make([]string, len(result))
	for i := 0; i < len(result); i++ {
		ret[i] = result[i].Name
	}

	return ret
}

func (gs *Gamershots) GetAllThemes() []string {
	result := []struct {
		Name string "_id"
	}{}
	gs.db.DB(MongoDbName).C(MongoGamesCollectionName).Pipe(
		[]bson.M{
			{"$unwind": "$themes"},
			{"$group": bson.M{"_id": "$themes"}},
			{"$sort": bson.M{"_id": 1}}}).All(&result)

	ret := make([]string, len(result))
	for i := 0; i < len(result); i++ {
		ret[i] = result[i].Name
	}

	return ret
}
