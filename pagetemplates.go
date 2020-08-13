package main

import (
	"bytes"
	"errors"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/html"
	"github.com/tdewolff/minify/svg"
	"html/template"
	"io"
	"log"
	"path/filepath"
	"strconv"
	"sync"
)

type PageTemplates struct {
	templates              *template.Template
	homePageTemplate       *template.Template
	recipePageTemplate     *template.Template
	editRecipePageTemplate *template.Template
	rwLock                 sync.RWMutex
}

func Dictionary(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("invalid dict call")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict keys must be strings")
		}
		dict[key] = values[i+1]
	}
	return dict, nil
}

func Sequence(values ...interface{}) ([]interface{}, error) {
	arr := make([]interface{}, len(values))
	for i, value := range values {
		arr[i] = value
	}
	return arr, nil
}

var funcMap = template.FuncMap{
	"dict": Dictionary,
	"seq":  Sequence,
}

var minifier = func() *minify.M {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.Add("text/html", &html.Minifier{
		KeepDocumentTags: true,
		KeepEndTags:      true,
		KeepQuotes:       true,
	})
	m.AddFunc("image/svg+xml", svg.Minify)
	return m
}()

func RenderAndMinify(t *template.Template, wr io.Writer, data interface{}) error {
	buf := bytes.Buffer{}
	err := t.Execute(&buf, data)
	if err != nil {
		return err
	}
	return minifier.Minify("text/html", wr, &buf)
}

func (t *PageTemplates) RenderHome(wr io.Writer, data interface{}) error {
	t.rwLock.RLock()
	te := t.homePageTemplate
	t.rwLock.RUnlock()
	return RenderAndMinify(te, wr, data)
}

func (t *PageTemplates) RenderEdit(wr io.Writer, data interface{}) error {
	t.rwLock.RLock()
	te := t.editRecipePageTemplate
	t.rwLock.RUnlock()
	return RenderAndMinify(te, wr, data)
}

func (t *PageTemplates) RenderRecipe(wr io.Writer, data interface{}) error {
	t.rwLock.RLock()
	te := t.recipePageTemplate
	t.rwLock.RUnlock()
	return RenderAndMinify(te, wr, data)
}

func (t *PageTemplates) Load(folder string, pattern string) error {
	t.rwLock.Lock()
	defer t.rwLock.Unlock()

	t.templates = template.New("_")
	t.templates.Funcs(funcMap)
	if _, err := t.templates.ParseGlob(filepath.Join(folder, pattern)); err != nil {
		return err
	}

	t.homePageTemplate = t.templates.Lookup("home.html")
	if t.homePageTemplate == nil {
		return errors.New("home page template missing")
	}

	t.recipePageTemplate = t.templates.Lookup("recipe-page.html")
	if t.recipePageTemplate == nil {
		return errors.New("recipe page template missing")
	}

	t.editRecipePageTemplate = t.templates.Lookup("edit-recipe-page.html")
	if t.editRecipePageTemplate == nil {
		return errors.New("edit recipe page template missing")
	}
	return nil
}

type PageRenderer struct {
	BaseUrl   string
	Templates *PageTemplates
}

func (r *PageRenderer) RenderHome(w io.Writer, recipes map[string]*RawRecipe) error {
	info := HomePageRenderInfo{
		PageRenderInfoBase: PageRenderInfoBase{
			BaseUrl: r.BaseUrl,
			Title:   "Rezepte",
		},
		Recipes: recipes,
	}
	log.Print("Rendering home page")
	return r.Templates.RenderHome(w, &info)
}

func (r *PageRenderer) RenderRecipe(w io.Writer, rid string, recipe *RawRecipe) error {
	baked := recipe.BakeRecipe()
	page := PageRenderInfo{
		Recipe: baked,
		PageRenderInfoBase: PageRenderInfoBase{
			BaseUrl: r.BaseUrl,
			Title:   baked.Name,
			Id:      rid,
		},
	}
	log.Print("Rendering recipe page ", strconv.Quote(rid))
	return r.Templates.RenderRecipe(w, &page)
}

func (r *PageRenderer) RenderEditRecipe(w io.Writer, rid string, recipe *RawRecipe) error {
	page := EditPageRenderInfo{
		Recipe: recipe,
		PageRenderInfoBase: PageRenderInfoBase{
			BaseUrl: r.BaseUrl,
			Title:   "Bearbeiten: " + recipe.Name,
			Id:      rid,
		},
	}
	log.Print("Rendering recipe edit page ", strconv.Quote(rid))
	return r.Templates.RenderEdit(w, &page)
}

func (r *PageRenderer) RenderCreate(w io.Writer) error {
	page := PageRenderInfo{
		PageRenderInfoBase: PageRenderInfoBase{
			BaseUrl: r.BaseUrl,
			Title:   "Neues Rezept",
		},
	}
	log.Print("Rendering create page")
	return r.Templates.RenderEdit(w, &page)
}
