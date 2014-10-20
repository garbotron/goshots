package goshots

func RenderFiltersPage(genericData *RendererData, showNotFound bool) error {
	data := FiltersRendererData{RendererData: *genericData}

	data.ReturnPage = data.Request.FormValue("return")
	if data.ReturnPage == "" {
		data.ReturnPage = "main"
	}

	data.ShowNotFoundPopup = showNotFound

	data.DisplayFilters = make([]DisplayFilter, len(data.Filters))
	for i, filter := range data.Filters {

		data.DisplayFilters[i].Filter = filter
		data.DisplayFilters[i].IsEnabled = data.FilterValues[i].Enabled
		data.DisplayFilters[i].IsFirstInRow = i%2 == 0
		data.DisplayFilters[i].IsLastInRow = i%2 == 1 || i == len(data.Filters)-1

		switch filter.Type() {
		case FilterTypeNumber:
			data.DisplayFilters[i].IsNumber = true
			if len(data.FilterValues[i].Values) >= 1 {
				data.DisplayFilters[i].NumValue1 = data.FilterValues[i].Values[0]
			}

		case FilterTypeNumberRange:
			data.DisplayFilters[i].IsNumberRange = true
			if len(data.FilterValues[i].Values) >= 2 {
				data.DisplayFilters[i].NumValue1 = data.FilterValues[i].Values[0]
				data.DisplayFilters[i].NumValue2 = data.FilterValues[i].Values[1]
			}

		case FilterTypeSelectOne:
			data.DisplayFilters[i].IsRadio = true
			opts, err := extractDisplayOptions(data.Provider, filter, data.FilterValues[i].Values)
			if err != nil {
				return err
			}
			data.DisplayFilters[i].Options = opts

		case FilterTypeSelectMany:
			data.DisplayFilters[i].IsMultiSelect = true
			opts, err := extractDisplayOptions(data.Provider, filter, data.FilterValues[i].Values)
			if err != nil {
				return err
			}
			data.DisplayFilters[i].Options = opts
		}
	}

	return RenderTemplate("filters.goshots", data.Writer, &data)
}

type FiltersRendererData struct {
	RendererData
	DisplayFilters    []DisplayFilter
	ReturnPage        string
	ShowNotFoundPopup bool
}

type DisplayOption struct {
	Name       string
	IsSelected bool
}

type DisplayFilter struct {
	Filter        Filter
	IsEnabled     bool
	IsNumber      bool
	IsNumberRange bool
	IsRadio       bool
	IsMultiSelect bool
	IsFirstInRow  bool
	IsLastInRow   bool
	Options       []DisplayOption
	NumValue1     int
	NumValue2     int
}

func inSlice(x int, slice []int) bool {
	for i := range slice {
		if x == slice[i] {
			return true
		}
	}
	return false
}

func extractDisplayOptions(
	provider Provider,
	filter Filter,
	filterValues []int) ([]DisplayOption, error) {

	optionNames, err := filter.Names(provider)
	if err != nil {
		return nil, err
	}

	options := make([]DisplayOption, len(optionNames))
	for i, name := range optionNames {
		options[i].Name = name
		options[i].IsSelected = inSlice(i, filterValues)
	}

	return options, nil
}
