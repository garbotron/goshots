package goshots

import (
	"html/template"
)

func RenderMainPage(genericData *RendererData) error {
	data := MainRendererData{RendererData: *genericData}

	elem, err := data.Provider.RandomElem(&data.FilterValues)
	if err != nil {
		if IsElemNotFoundError(err) {
			return RenderFiltersPage(genericData, true)
		}
		return err
	}

	solution, err := data.Provider.ElemSolution(elem)
	if err != nil {
		return err
	}

	content, err := data.Provider.RenderContentHtml(elem)
	if err != nil {
		return err
	}

	data.Solution = solution
	data.Content = content

	return RenderTemplate("main.goshots", data.Writer, &data)
}

type MainRendererData struct {
	RendererData
	Solution string
	Content  template.HTML
}
