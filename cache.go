package main

import (
	"io"
	"strings"
	"sync"
)

type PageCacheEntry struct {
	value *string
	lock  sync.RWMutex
}

type RecipePagesCacheEntry struct {
	recipePage PageCacheEntry
	editPage   PageCacheEntry
}

type PageCacheMap struct {
	cache map[string]*RecipePagesCacheEntry
	lock  sync.RWMutex
}

type RenderCache struct {
	renderer *PageRenderer
	database *RecipesDatabase

	home    PageCacheEntry
	create  PageCacheEntry
	recipes PageCacheMap
}

func NewRenderCache(renderer *PageRenderer, database *RecipesDatabase) *RenderCache {
	c := &RenderCache{
		renderer: renderer,
		database: database,
		recipes: PageCacheMap{
			cache: make(map[string]*RecipePagesCacheEntry),
		},
	}

	for k := range database.GetAll() {
		c.AddRecipe(k)
	}
	return c
}

type RenderFunction func(w io.Writer) error

func (e *PageCacheEntry) Invalidate() {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.value = nil
}

func (e *PageCacheEntry) Update(update RenderFunction) (string, error) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.value == nil {
		b := strings.Builder{}
		err := update(&b)
		if err != nil {
			return "", err
		}
		s := b.String()
		e.value = &s
	}
	return *e.value, nil
}

func (e *PageCacheEntry) GetOrUpdate(update RenderFunction) (string, error) {
	e.lock.RLock()
	if e.value == nil {
		e.lock.RUnlock()
		return e.Update(update)
	} else {
		defer e.lock.RUnlock()
		return *e.value, nil
	}
}

func (m *PageCacheMap) Get(rid string) (*RecipePagesCacheEntry, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	pageCache, ok := m.cache[rid]
	return pageCache, ok
}

func (m *PageCacheMap) Create(rid string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.cache[rid] = &RecipePagesCacheEntry{}
}

func (m *PageCacheMap) Remove(rid string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.cache, rid)
}

func (m *PageCacheMap) Invalidate(rid string) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	c, ok := m.cache[rid]
	if !ok {
		return
	}
	c.editPage.Invalidate()
	c.recipePage.Invalidate()
}

func (m *PageCacheMap) InvalidateAll() {
	for _, v := range m.cache {
		v.recipePage.Invalidate()
		v.editPage.Invalidate()
	}
}

// These functions need the database write lock //
func (c *RenderCache) InvalidateHome() {
	c.home.Invalidate()
}

func (c *RenderCache) InvalidateRecipe(rid string) {
	c.recipes.Invalidate(rid)
}

func (c *RenderCache) RemoveRecipe(rid string) {
	c.recipes.Remove(rid)
}

//////////////////////////////////////////////////

func (c *RenderCache) GetHome() (string, error) {
	return c.home.GetOrUpdate(func(w io.Writer) error {
		return c.renderer.RenderHome(w, c.database.GetAll())
	})
}

func (c *RenderCache) GetCreatePage() (string, error) {
	return c.create.GetOrUpdate(func(w io.Writer) error {
		return c.renderer.RenderCreate(w)
	})
}

func (c *RenderCache) GetRecipePage(rid string) (string, error, bool) {
	e, ok := c.recipes.Get(rid)
	if !ok {
		return "", nil, false
	}

	s, err := e.recipePage.GetOrUpdate(func(w io.Writer) error {
		r := c.database.GetMustExist(rid)
		return c.renderer.RenderRecipe(w, rid, r)
	})
	return s, err, true
}

func (c *RenderCache) GetRecipeEditPage(rid string) (string, bool, error) {
	e, ok := c.recipes.Get(rid)
	if !ok {
		return "", false, nil
	}
	s, err := e.editPage.GetOrUpdate(func(w io.Writer) error {
		r := c.database.GetMustExist(rid)
		return c.renderer.RenderEditRecipe(w, rid, r)
	})
	return s, true, err
}

func (c *RenderCache) AddRecipe(rid string) {
	c.recipes.Create(rid)
}

func (c *RenderCache) InvalidateAll() {
	c.recipes.InvalidateAll()
	c.home.Invalidate()
	c.create.Invalidate()
}
