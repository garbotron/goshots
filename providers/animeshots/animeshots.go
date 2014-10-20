package animeshots

import (
	"fmt"
	"github.com/garbotron/goshots/core"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"math/rand"
)

const MongoServerAddress = "localhost"
const MongoDbName = "animeshots"
const MongoTempDbName = "animeshots_wip"
const MongoShowsCollectionName = "shows"

type Animeshots struct {
	db *mgo.Session
}

type Show struct {
	Name           string
	Type           string
	Year           int
	HasYear        bool
	Tags           []string
	ScreenshotUrls []string
}

func (_ *Animeshots) ShortName() string {
	return "animeshots"
}

func (_ *Animeshots) PrettyName() string {
	return "Animeshots"
}

func (_ *Animeshots) Description() []template.HTML {
	return []template.HTML{
		"Animeshots is a drinking show for anime geeks.",
		"Check out a screenshot, see if you can name the show!",
		"The concept is simple, but I couldn't find another site on the net that does it.",
		"<b>If you can't name the show, take a drink!</b>",
		"<b>If you can, everyone else takes a drink and you go again!</b>",
	}
}

func (_ *Animeshots) Title() string {
	return "Animeshots: Name That Screenshot!"
}

func (_ *Animeshots) Prompt() string {
	return "What show is this?"
}

func (as *Animeshots) Load() error {
	if as.db != nil {
		as.db.Close()
	}
	var err error
	as.db, err = mgo.Dial(MongoServerAddress)
	return err
}

func (as *Animeshots) RandomElem(filterValues *goshots.FilterValues) (interface{}, error) {

	db := as.db.DB(MongoDbName).C(MongoShowsCollectionName)

	filters := AnimeshotsFilters()
	exprs := []bson.M{}
	for i, fv := range *filterValues {
		if fv.Enabled {
			for _, expr := range filters[i].Apply(as, fv.Values) {
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

func (as *Animeshots) ElemSolution(elem interface{}) (string, error) {

	id := elem.(bson.ObjectId)
	db := as.db.DB(MongoDbName).C(MongoShowsCollectionName)
	show := Show{}
	err := db.FindId(id).One(&show)
	if err != nil {
		return "", err
	}
	ret := show.Name
	if show.HasYear {
		ret = fmt.Sprintf("%s (%d)", ret, show.Year)
	}
	return ret, nil
}

func (as *Animeshots) RenderContentHtml(elem interface{}) (template.HTML, error) {

	id := elem.(bson.ObjectId)
	db := as.db.DB(MongoDbName).C(MongoShowsCollectionName)
	show := Show{}
	err := db.FindId(id).One(&show)
	if err != nil {
		return "", err
	}

	elemIdx := rand.Int() % len(show.ScreenshotUrls)

	str := "<div style=\"width:100%;height:100%;display:table;background-image:url(" +
		show.ScreenshotUrls[elemIdx] +
		");background-repeat:no-repeat;background-size:contain;background-position:center;-webkit-background-size:contain;-moz-background-size:contain;-o-background-size:contain;background-size:contain;\"> </div>"
	return template.HTML(str), nil
}

func (as *Animeshots) GetAllTypes() []string {
	result := []struct {
		Name string "_id"
	}{}
	as.db.DB(MongoDbName).C(MongoShowsCollectionName).Pipe(
		[]bson.M{
			{"$group": bson.M{"_id": "$type"}},
			{"$sort": bson.M{"_id": 1}}}).All(&result)

	ret := make([]string, len(result))
	for i := 0; i < len(result); i++ {
		ret[i] = result[i].Name
	}

	return ret
}

func (as *Animeshots) GetAllTags() []string {
	result := []struct {
		Name string "_id"
	}{}
	as.db.DB(MongoDbName).C(MongoShowsCollectionName).Pipe(
		[]bson.M{
			{"$unwind": "$tags"},
			{"$group": bson.M{"_id": "$tags"}},
			{"$sort": bson.M{"_id": 1}}}).All(&result)

	ret := make([]string, len(result))
	for i := 0; i < len(result); i++ {
		ret[i] = result[i].Name
	}

	return ret
}
