package main

import (
	"context"
	"encoding/json"
	"github.com/dlclark/regexp2"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
	"github.com/urfave/cli"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type RawRecipeWithId struct {
	Recipe *RawRecipe
	Id     string
}

func ReadRecipes(folder string) ([]RawRecipeWithId, error) {
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	recipes := make([]RawRecipeWithId, len(files))
	offset := 0
	for _, file := range files {
		name := file.Name()
		ext := filepath.Ext(name)
		if ext != ".json" {
			continue
		}
		fullPath := filepath.Join(folder, name)
		recipe, err := ParseFile(fullPath)
		if err != nil {
			return nil, err
		}

		recipes[offset] = RawRecipeWithId{
			Recipe: recipe,
			Id:     strings.TrimSuffix(name, filepath.Ext(name)),
		}
		offset++
	}
	return recipes, nil
}

type PageRenderInfoBase struct {
	BaseUrl string
	Title   string
	Id      string
}

type PageRenderInfo struct {
	PageRenderInfoBase
	Recipe *BakedRecipe
}

type EditPageRenderInfo struct {
	PageRenderInfoBase
	Recipe *RawRecipe
}

func ReadRecipeFromResponse(body io.Reader) (*RawRecipe, error) {
	bodyBytes, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	var recipeRead RawRecipe
	err = json.Unmarshal(bodyBytes, &recipeRead)
	if err != nil {
		return nil, err
	}
	recipeRead.Clean()
	return &recipeRead, nil
}

type RecipesContext struct {
	Templates    *PageTemplates
	Recipes      *Recipes
	Secure       bool
	BaseUrl      string
	TokenManager *TokenManager
	Watcher      *FileWatcher
	Renderer     *PageRenderer
}

type RecipesParams struct {
	templatesDirPath, templatesPattern, recipesDirPath, tokensPath string
	unsecure                                                       bool
	baseUrl                                                        string
}

type ServerParams struct {
	RecipesParams
	port     int
	address  string
	certFile string
	keyFile  string
}

func (ctx *RecipesContext) StartWatchTokenFile(file string) error {
	return ctx.Watcher.AddFileWatch(file, func(events []fsnotify.Event) {
		if e := FirstNonChmodIn(events); e == nil {
			return
		}

		log.Print("Reloading tokens file")
		if err := ctx.TokenManager.ReloadFromFile(file); err != nil {
			log.Print("Failed to reload tokens: ", err)
		}
	})
}

func (ctx *RecipesContext) StartWatchTemplates(folder string, pattern string) error {
	return ctx.Watcher.Add(folder, func(events []fsnotify.Event) {
		if e := FirstNonChmodIn(events); e == nil {
			return
		}

		log.Print("Reloading templates from \"", folder, "\"")
		if err := ctx.Templates.Load(folder, pattern); err != nil {
			log.Print("Error reloading templates: ", err)
		}
		ctx.Recipes.InvalidateAll()
	})
}

func Init(params *RecipesParams) (*RecipesContext, error) {
	templates := PageTemplates{}
	err := templates.Load(params.templatesDirPath, params.templatesPattern)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(params.recipesDirPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	recipesList, err := ReadRecipes(params.recipesDirPath)
	if err != nil {
		return nil, err
	}

	recipes := make(map[string]*RawRecipe)
	for _, r := range recipesList {
		recipes[r.Id] = r.Recipe
	}

	tokens := NewTokenManager("token")
	if _, err := os.Stat(params.tokensPath); os.IsNotExist(err) {
		b, err := json.Marshal(make(map[Identifier]Token))
		if err != nil {
			return nil, err
		}
		if err = ioutil.WriteFile(params.tokensPath, b, 0666); err != nil {
			return nil, err
		}
	}
	err = tokens.ReloadFromFile(params.tokensPath)
	if err != nil {
		return nil, err
	}

	watcher, err := NewFileWatcher(1 * time.Second)
	if err != nil {
		return nil, err
	}

	database := &RecipesDatabase{
		Path:    params.recipesDirPath,
		recipes: recipes,
	}

	renderer := &PageRenderer{
		BaseUrl:   params.baseUrl,
		Templates: &templates,
	}

	ctx := &RecipesContext{
		Templates:    &templates,
		TokenManager: tokens,
		Watcher:      watcher,
		Secure:       !params.unsecure,
		Renderer:     renderer,
		Recipes: &Recipes{
			Database: database,
			Cache:    NewRenderCache(renderer, database),
		},
	}

	if err = ctx.StartWatchTemplates(params.templatesDirPath, params.templatesPattern); err != nil {
		return nil, err
	}
	if err = ctx.StartWatchTokenFile(params.tokensPath); err != nil {
		return nil, err
	}

	return ctx, nil
}

type HomePageRenderInfo struct {
	PageRenderInfoBase
	Recipes map[string]*RawRecipe
}

func (ctx *RecipesContext) Authenticate(r *http.Request) bool {
	_, ok := ctx.TokenManager.GetFromRequest(r)
	if !ok {
		return false
	}

	return true
}

func AuthenticatedTokenIdentifier(r *http.Request) (Identifier, bool) {
	if v := r.Context().Value("token-identifier"); v != nil {
		return v.(Identifier), true
	}
	return DefaultIdentifier, false
}

func (ctx *RecipesContext) HandleAuthenticate(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := ctx.TokenManager.GetFromRequest(r)
		if !ok {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		c := context.WithValue(r.Context(), "token-identifier", id)
		h.ServeHTTP(w, r.WithContext(c))
	})
}

func (ctx *RecipesContext) HandleAuthentication(w http.ResponseWriter, r *http.Request) {
	var i Identifier
	if _, err := r.Cookie(ctx.TokenManager.CookieName); err != nil {
		i = "Cookie not set"
	} else if id, ok := ctx.TokenManager.GetFromRequest(r); ok {
		i = id
	} else {
		i = "Unknown user"
	}

	if _, err := w.Write([]byte(i)); err != nil {
		log.Panic(err)
	}
}

func (ctx *RecipesContext) HandleAuthenticationSet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]
	cookie := http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		MaxAge:   2147483647,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   ctx.Secure,
	}
	http.SetCookie(w, &cookie)
	if _, err := w.Write([]byte("Success")); err != nil {
		log.Panic(err)
	}
}

var identifierRegex = regexp2.MustCompile("^[A-Za-z0-9 _-]+$", regexp2.Singleline)

func (ctx *RecipesContext) HandleAuthenticationGenerate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	identifier := strings.TrimSpace(vars["identifier"])
	if ok, _ := identifierRegex.MatchString(identifier); !ok {
		http.Error(w, "Identifier contains invalid characters", 400)
		log.Print("Identifier ", strconv.Quote(identifier), " contains invalid characters")
		return
	}

	token, err := ctx.TokenManager.GenerateFor(Identifier(identifier))
	if err != nil {
		http.Error(w, "Failed to generate a token", 400)
		log.Print("Failed to generate a token for ", strconv.Quote(identifier), ": ", err)
		return
	}
	if _, err = w.Write([]byte(token)); err != nil {
		log.Panic(err)
	}
}

func HandleRenderError(err error) {
	if err != nil {
		panic(err)
	}
}

func WriteString(w http.ResponseWriter, s string) {
	_, err := w.Write([]byte(s))
	HandleRenderError(err)
}

func (ctx *RecipesContext) HandleHome(w http.ResponseWriter, _ *http.Request) {
	s, err := ctx.Recipes.GetHomePage()
	if err != nil {
		HandleRenderError(err)
	}
	WriteString(w, s)
}

func (ctx *RecipesContext) HandleShow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rid := vars["recipe"]
	s, err, ok := ctx.Recipes.GetRecipePage(rid)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		HandleRenderError(err)
	}
	WriteString(w, s)
}

func (ctx *RecipesContext) HandleCreate(w http.ResponseWriter, _ *http.Request) {
	s, err := ctx.Recipes.GetCreatePage()
	if err != nil {
		HandleRenderError(err)
	}
	WriteString(w, s)
}

func ReadRecipeRequestResponse(w http.ResponseWriter, r *http.Request) (recipe *RawRecipe, rid string, ok bool) {
	recipe, err := ReadRecipeFromResponse(r.Body)
	if err != nil {
		http.Error(w, "Failed to read edit post request body", 400)
		log.Print("Failed to read edit post request body ", err)
		return nil, "", false
	}

	rid = TransformToIdString(strings.TrimSpace(recipe.Name))
	if len(rid) == 0 {
		http.Error(w, "Can't create a recipe with an empty id", 400)
		log.Print("Can't create a recipe with an empty id")
		return nil, "", false
	}
	return recipe, rid, true
}

func (ctx *RecipesContext) RedirectTo(w http.ResponseWriter, r *http.Request, add string) {
 	http.Redirect(w, r, ctx.BaseUrl+add, http.StatusSeeOther)
}

func (ctx *RecipesContext) RedirectToRecipe(w http.ResponseWriter, r *http.Request, rid string) {
	ctx.RedirectTo(w, r, "/recipe/"+rid)
}

func ErrorPlaylistAlreadyExists(w http.ResponseWriter, rid string) {
	http.Error(w, "A playlist with this id already exists", 400)
	log.Print("A playlist with the id \"", rid, "\" already exists")
}

func (ctx *RecipesContext) HandleCreateResponse(w http.ResponseWriter, r *http.Request) {
	id, _ := AuthenticatedTokenIdentifier(r)
	recipe, rid, ok := ReadRecipeRequestResponse(w, r)
	if !ok {
		return
	}

	alreadyContained, err := ctx.Recipes.AddRecipe(rid, recipe)
	if alreadyContained {
		ErrorPlaylistAlreadyExists(w, rid)
		return
	}

	if err != nil {
		http.Error(w, "Failed to add playlist", 400)
		log.Print("Failed to add playlist \"", rid, "\": ", err)
		return
	}

	log.Print("Created recipe \"", rid, "\" (", id, ")")
	ctx.RedirectToRecipe(w, r, rid)
}

func (ctx *RecipesContext) HandleEdit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rid := vars["recipe"]

	s, exists, err := ctx.Recipes.GetRecipeEditPage(rid)
	if !exists {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		HandleRenderError(err)
	}
	WriteString(w, s)
}

func (ctx *RecipesContext) HandleEditResponse(w http.ResponseWriter, r *http.Request) {
	identifier, _ := AuthenticatedTokenIdentifier(r)
	vars := mux.Vars(r)
	oldRid := vars["recipe"]

	recipe, rid, ridOk := ReadRecipeRequestResponse(w, r)
	if !ridOk {
		return
	}

	alreadyContained, err := ctx.Recipes.ReplaceRecipe(rid, oldRid, recipe)
	if alreadyContained {
		ErrorPlaylistAlreadyExists(w, rid)
		return
	}
	if err != nil {
		http.Error(w, "Failed to replace recipe", 400)
		log.Print("Failed to replace recipe ", strconv.Quote(oldRid), " with ", strconv.Quote(rid), ": ", err)
		return
	}

	log.Print("Updated recipe ", strconv.Quote(rid), " (", identifier, ")")
	ctx.RedirectToRecipe(w, r, rid)
}

func (ctx *RecipesContext) HandleDeleteResponse(w http.ResponseWriter, r *http.Request) {
	identifier, _ := AuthenticatedTokenIdentifier(r)
	vars := mux.Vars(r)
	rid := vars["recipe"]

	exists, err := ctx.Recipes.RemoveRecipe(rid)
	if !exists {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to replace recipe", 400)
		return
	}
	log.Print("Deleted recipe ", strconv.Quote(rid), " (", identifier, ")")
	ctx.RedirectTo(w, r, "/")
}

func IsNotOk(r rune) bool {
	return r < 32 || r >= 127 || !(unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) || r == '-')
}

var transformer = transform.Chain(norm.NFKD, runes.Remove(runes.Predicate(IsNotOk)))

type ReplaceFunction func(rune) []rune

func ReplaceWithMany(s string, replace ReplaceFunction) string {
	builder := strings.Builder{}
	builder.Grow(len(s))
	for _, r := range s {
		for _, rep := range replace(r) {
			builder.WriteRune(rep)
		}
	}
	return builder.String()
}

func ReplaceGermanUmlauts(r rune) []rune {
	lower := unicode.ToLower(r)
	switch lower {
	case 'ä':
		return []rune{'a', 'e'}
	case 'ö':
		return []rune{'o', 'e'}
	case 'ü':
		return []rune{'u', 'e'}
	case 'ß':
		return []rune{'s', 's'}
	default:
		return []rune{ lower }
	}
}

func ReplaceSpaceAndCollapse(s string, replacement rune) string {
	builder := strings.Builder{}
	builder.Grow(len(s))
	lastSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) || r == replacement {
			if !lastSpace {
				builder.WriteRune(replacement)
			}
			lastSpace = true
		} else {
			builder.WriteRune(r)
			lastSpace = false
		}
	}
	return builder.String()
}

func TransformToIdString(s string) string {
	rep := ReplaceWithMany(s, ReplaceGermanUmlauts)
	str, _, _ := transform.String(transformer, rep)
	withoutSpaces := ReplaceSpaceAndCollapse(str, '-')
	return withoutSpaces
}

func InitHandlers(r *mux.Router, ctx *RecipesContext) {
	scheme := "http"
	if ctx.Secure {
		scheme = "https"
	}

	r.HandleFunc("/", ctx.HandleHome).Schemes(scheme)
	r.HandleFunc("/recipe/{recipe}", ctx.HandleShow).Methods("GET").Schemes(scheme)
	r.HandleFunc("/create", ctx.HandleCreate).Methods("GET").Schemes(scheme)
	r.HandleFunc("/edit/{recipe}", ctx.HandleEdit).Methods("GET").Schemes(scheme)

	r.HandleFunc("/authentication", ctx.HandleAuthentication).Methods("GET").Schemes(scheme)
	r.HandleFunc("/authentication/generate/{identifier}", ctx.HandleAuthenticationGenerate).Methods("GET").Schemes(scheme)
	r.HandleFunc("/authentication/set/{token}", ctx.HandleAuthenticationSet).Methods("GET").Schemes(scheme)

	r.Handle("/create", ctx.HandleAuthenticate(http.HandlerFunc(ctx.HandleCreateResponse))).Methods("POST").Schemes(scheme)
	r.Handle("/delete/{recipe}", ctx.HandleAuthenticate(http.HandlerFunc(ctx.HandleDeleteResponse))).Methods("POST").Schemes(scheme)
	r.Handle("/edit/{recipe}", ctx.HandleAuthenticate(http.HandlerFunc(ctx.HandleEditResponse))).Methods("POST").Schemes(scheme)

	fs := http.FileServer(http.Dir("static/"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs)).Methods("GET").Schemes(scheme)
}

func RunServer(params *ServerParams) error {
	ctx, err := Init(&params.RecipesParams)
	if err != nil {
		return err
	}
	defer ctx.Watcher.Stop()

	r := mux.NewRouter()
	InitHandlers(r, ctx)

	addr := params.address + ":" + strconv.Itoa(params.port)
	log.Print("Starting server on ", addr)
	if params.unsecure {
		return http.ListenAndServe(addr, r)
	} else {
		return http.ListenAndServeTLS(addr, params.certFile, params.keyFile, r)
	}
}

func main() {
	params := ServerParams{}

	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "address",
				Value:       "[::]",
				Usage:       "server address",
				Destination: &params.address,
			},
			&cli.IntFlag{
				Name:        "port",
				Value:       8000,
				Usage:       "server port",
				Destination: &params.port,
			},
			&cli.StringFlag{
				Name:        "base-url",
				Value:       "",
				Usage:       "base url",
				Destination: &params.baseUrl,
			},
			&cli.StringFlag{
				Name:        "templates-dir",
				Value:       "templates",
				Usage:       "path to parse the template files from",
				Destination: &params.templatesDirPath,
			},
			&cli.StringFlag{
				Name:        "templates-pattern",
				Value:       "*.html",
				Usage:       "the file pattern for template files in the templates folder",
				Destination: &params.templatesPattern,
			},
			&cli.StringFlag{
				Name:        "tokens",
				Value:       "tokens.json",
				Usage:       "tokens file to use for authorization",
				Destination: &params.tokensPath,
			},
			&cli.StringFlag{
				Name:        "recipes",
				Value:       "recipes",
				Usage:       "folder to write the recipes to",
				Destination: &params.recipesDirPath,
			},
			&cli.BoolFlag{
				Name:        "disable-https",
				Value:       false,
				Usage:       "tokens file to use for authorization",
				Destination: &params.unsecure,
			},
			&cli.StringFlag{
				Name:        "cert",
				Value:       "localhost.crt",
				Usage:       "certificate file for tls",
				Destination: &params.certFile,
			},
			&cli.StringFlag{
				Name:        "key",
				Value:       "localhost.key",
				Usage:       "key file for tls",
				Destination: &params.keyFile,
			},
		},
		Name:  "recipes-server",
		Usage: "start the recipes server",
		Action: func(c *cli.Context) error {
			return RunServer(&params)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Panic(err)
	}
}
