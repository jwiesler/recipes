package main

import (
	"encoding/json"
	"github.com/gomarkdown/markdown"
	"github.com/microcosm-cc/bluemonday"
	"html/template"
	"io/ioutil"
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

type BakedRecipe struct {
	Name                string
	ImagePath           string
	Description         string
	IngredientsSections []IngredientsSection
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

	return ioutil.WriteFile(file, b, 0)
}

var policy = bluemonday.UGCPolicy()

func BakeString(s string) template.HTML {
	return template.HTML(policy.SanitizeBytes(markdown.ToHTML([]byte(s), nil, nil)))
}

func (r *RawRecipe) BakeRecipe() *BakedRecipe {
	return &BakedRecipe{
		Name:                r.Name,
		ImagePath:           r.ImagePath,
		Description:         r.Description,
		IngredientsSections: r.IngredientsSections,
		Instructions:        BakeString(r.Instructions),
		Source:              BakeString(r.Source),
	}
}
