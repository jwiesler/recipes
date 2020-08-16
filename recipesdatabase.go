package main

import (
	"go.uber.org/zap"
	"os"
	"path"
)

type RecipesDatabase struct {
	Path    string
	recipes map[string]*RawRecipe
}

func (m *RecipesDatabase) makePath(id string) string {
	return path.Join(m.Path, id+".json")
}

func (m *RecipesDatabase) writeRecipe(id string, recipe *RawRecipe) error {
	return recipe.WriteToFile(m.makePath(id))
}

func (m *RecipesDatabase) insertRecipe(id string, recipe *RawRecipe) error {
	m.recipes[id] = recipe
	return m.writeRecipe(id, recipe)
}

func (m *RecipesDatabase) removeIfExists(id string) (bool, error) {
	if _, ok := m.recipes[id]; ok {
		delete(m.recipes, id)
		return true, os.Remove(m.makePath(id))
	}
	return false, nil
}

func (m *RecipesDatabase) Add(id string, recipe *RawRecipe) (alreadyContained bool, err error) {
	if _, ok := m.recipes[id]; ok {
		return true, nil
	}
	return false, m.insertRecipe(id, recipe)
}

func (m *RecipesDatabase) Get(id string) (*RawRecipe, bool) {
	r, ok := m.recipes[id]
	return r, ok
}

func (m *RecipesDatabase) GetMustExist(id string) *RawRecipe {
	r, ok := m.Get(id)
	if !ok {
		logger.Panic("Recipe should be in database", zap.String("id", id))
	}
	return r
}

func (m *RecipesDatabase) GetAll() map[string]*RawRecipe {
	return m.recipes
}

func (m *RecipesDatabase) Remove(id string) (bool, error) {
	return m.removeIfExists(id)
}

func (m *RecipesDatabase) UpdateRecipe(rid string, recipe *RawRecipe) error {
	return m.insertRecipe(rid, recipe)
}
