package main

import (
	"sync"
)

type Recipes struct {
	Database *RecipesDatabase
	Cache    *RenderCache
	lock     sync.RWMutex
}

func (c *Recipes) addRecipe(rid string, recipe *RawRecipe) (alreadyContained bool, err error) {
	alreadyContained, err = c.Database.Add(rid, recipe)
	if alreadyContained || err != nil {
		return alreadyContained, err
	}
	c.Cache.AddRecipe(rid)
	c.Cache.InvalidateHome()
	return false, nil
}

func (c *Recipes) removeRecipe(rid string) error {
	err := c.Database.Remove(rid)
	if err != nil {
		return err
	}
	c.Cache.RemoveRecipe(rid)
	c.Cache.InvalidateHome()
	return nil
}

func (c *Recipes) AddRecipe(rid string, recipe *RawRecipe) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.addRecipe(rid, recipe)
}

func (c *Recipes) ReplaceRecipe(rid, oldRId string, recipe *RawRecipe) (alreadyContained bool, err error) {
	if rid != oldRId {
		c.lock.Lock()
		defer c.lock.Unlock()
		alreadyContained, err := c.addRecipe(rid, recipe)
		if alreadyContained || err != nil {
			return alreadyContained, err
		}
		return false, c.removeRecipe(oldRId)
	} else {
		c.lock.RLock()
		defer c.lock.RUnlock()
		if err := c.Database.UpdateRecipe(rid, recipe); err != nil {
			return false, err
		}
		c.Cache.InvalidateRecipe(rid)
		c.Cache.InvalidateHome()
		return false, nil
	}
}

func (c *Recipes) RemoveRecipe(rid string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.removeRecipe(rid)
}

//////////////////////////////////////////////////

func (c *Recipes) GetHomePage() (string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.Cache.GetHome()
}

func (c *Recipes) GetCreatePage() (string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.Cache.GetCreatePage()
}

func (c *Recipes) GetRecipePage(rid string) (string, error, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.Cache.GetRecipePage(rid)
}

func (c *Recipes) GetRecipeEditPage(rid string) (string, bool, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.Cache.GetRecipeEditPage(rid)
}

func (c *Recipes) InvalidateAll() {
	c.lock.RLock()
	defer c.lock.RUnlock()
	c.Cache.InvalidateAll()
}
