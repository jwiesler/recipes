package main

import (
	"encoding/json"
	"github.com/gomarkdown/markdown"
	"github.com/microcosm-cc/bluemonday"
	"html/template"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
)

type Ingredient struct {
	Name   string
	Amount string
	Unit   string `json:"Unit,omitempty"`
}

type IngredientsSection struct {
	Heading     string
	Ingredients []Ingredient
}

type RawRecipe struct {
	Name                string
	ImagePath           string
	Description         string
	IngredientsSections []IngredientsSection
	Instructions        string
	Source              string
}

type IngredientSummaryKey struct {
	Name string
	Unit string
}

type IngredientSummary struct {
	Name string
	Unit string
	Amount float64
	RecipeOffset int
}

type BakedRecipe struct {
	Name                string
	ImagePath           string
	Description         string
	IngredientsSections []IngredientsSection
	IngredientSummaries []IngredientSummary
	Instructions        template.HTML
	Source              template.HTML
}

func ParseFile(file string) (*RawRecipe, error) {
	c, e := ioutil.ReadFile(file)
	if e != nil {
		return nil, e
	}
	var recipeRead RawRecipe
	e = json.Unmarshal(c, &recipeRead)
	return &recipeRead, e
}

func (r *RawRecipe) WriteToFile(file string) error {
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}

	return ioutil.WriteFile(file, b, 0666)
}

var policy = bluemonday.UGCPolicy()

func BakeString(s string) template.HTML {
	return template.HTML(policy.SanitizeBytes(markdown.ToHTML([]byte(s), nil, nil)))
}

type ingredientSummariesSorter []IngredientSummary

func (s ingredientSummariesSorter) Len() int {
	return len(s)
}

func (s ingredientSummariesSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ingredientSummariesSorter) Less(i, j int) bool {
	return s[i].RecipeOffset < s[j].RecipeOffset
}

func MakeIngredientSummaries(sections []IngredientsSection) []IngredientSummary {
	ingredients := make(map[IngredientSummaryKey]*IngredientSummary)
	for _, section := range sections {
		for _, ingredient := range section.Ingredients {
			amount, err := strconv.ParseFloat(ingredient.Amount, 64)
			if err != nil {
				continue
			}

			key := IngredientSummaryKey{
				Name: ingredient.Name,
				Unit: ingredient.Unit,
			}

			if i, ok := ingredients[key]; ok {
				i.Amount += amount
			} else {
				ingredients[key] = &IngredientSummary{
					Name: ingredient.Name,
					Unit: ingredient.Unit,
					Amount: amount,
					RecipeOffset: len(ingredients),
				}
			}
		}
	}

	res := make([]IngredientSummary, 0, len(ingredients))
	for _, in := range ingredients {
		res = append(res, *in)
	}
	sort.Sort(ingredientSummariesSorter(res))
	return res
}

func (r *RawRecipe) BakeRecipe() *BakedRecipe {
	return &BakedRecipe{
		Name:                r.Name,
		ImagePath:           r.ImagePath,
		Description:         r.Description,
		IngredientsSections: r.IngredientsSections,
		IngredientSummaries: MakeIngredientSummaries(r.IngredientsSections),
		Instructions:        BakeString(r.Instructions),
		Source:              BakeString(r.Source),
	}
}

func (r *RawRecipe) Clean() {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	for _, section := range r.IngredientsSections {
		section.Heading = strings.TrimSpace(section.Heading)
		for _, ingredient := range section.Ingredients {
			ingredient.Name = strings.TrimSpace(ingredient.Name)
			ingredient.Unit = strings.TrimSpace(ingredient.Unit)
			ingredient.Amount = strings.TrimSpace(ingredient.Amount)
		}
	}
}
