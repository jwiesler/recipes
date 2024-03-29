package main

import (
	"encoding/json"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var logger *zap.Logger

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
	templatesDirPath string
	templatesPattern string
	recipesDirPath   string
	tokensPath       string
	tokensKeyPath    string
	unsecure         bool
	baseUrl          string
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

		logger.Debug("Reloading tokens file", zap.String("path", file))
		if err := ctx.TokenManager.ReloadFromFile(file); err != nil {
			logger.Warn("Failed to reload tokens", zap.Error(err))
		}
	})
}

func (ctx *RecipesContext) StartWatchTemplates(folder string, pattern string) error {
	return ctx.Watcher.Add(folder, func(events []fsnotify.Event) {
		if e := FirstNonChmodIn(events); e == nil {
			return
		}

		logger.Debug("Reloading templates", zap.String("path", folder))
		if err := ctx.Templates.Load(folder, pattern); err != nil {
			logger.Warn("Failed to reload templates", zap.Error(err))
		}
		ctx.Recipes.InvalidateAll()
	})
}

func CreateTokensManager(params *RecipesParams) (*TokenManager, error) {
	logger.Info("Loading tokens key", zap.String("path", params.tokensKeyPath))
	key, err := ReadTokensKeyFile(params.tokensKeyPath)
	if err != nil {
		return nil, err
	}
	logger.Info("Loading tokens file", zap.String("path", params.tokensPath))
	if _, err := os.Stat(params.tokensPath); os.IsNotExist(err) {
		logger.Info("Tokens file not found, creating new")
		m := make(map[Identifier]Token)
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		if err = ioutil.WriteFile(params.tokensPath, b, 0666); err != nil {
			return nil, err
		}
	}
	tokens := NewTokenManager("token", key)
	err = tokens.ReloadFromFile(params.tokensPath)
	if err != nil {
		return nil, err
	}
	logger.Info("Tokens loaded", zap.Int("count", len(tokens.tokens)))
	return tokens, nil
}

func CreateRecipesDatabase(params *RecipesParams) (*RecipesDatabase, error) {
	logger.Info("Loading recipes", zap.String("path", params.recipesDirPath))
	err := os.MkdirAll(params.recipesDirPath, os.ModePerm)
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
	logger.Info("Recipes loaded", zap.Int("count", len(recipes)))
	return &RecipesDatabase{
		Path:    params.recipesDirPath,
		recipes: recipes,
	}, nil
}

func Init(params *RecipesParams) (*RecipesContext, error) {
	tokens, err := CreateTokensManager(params)
	if err != nil {
		return nil, err
	}
	templates := PageTemplates{}
	logger.Info("Loading templates ", zap.String("path", params.templatesDirPath), zap.String("pattern", params.templatesPattern))
	err = templates.Load(params.templatesDirPath, params.templatesPattern)
	if err != nil {
		return nil, err
	}
	database, err := CreateRecipesDatabase(params)
	if err != nil {
		return nil, err
	}
	watcher, err := NewFileWatcher(1 * time.Second)
	if err != nil {
		return nil, err
	}

	logger.Info("Creating renderer", zap.String("base-url", params.baseUrl))
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

type AuthenticatedHandler func(http.ResponseWriter, *http.Request, Identifier)

func (ctx *RecipesContext) HandleAuthenticate(h AuthenticatedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identifier, ok := ctx.TokenManager.GetFromRequest(r)
		if !ok {
			http.Error(w, "access-denied", http.StatusForbidden)
			return
		}
		h(w, r, identifier)
	})
}

func (ctx *RecipesContext) HandleAuthentication(w http.ResponseWriter, r *http.Request) {
	var user Identifier
	var cookieSet bool
	token, cookieSet := ctx.TokenManager.GetTokenFromRequest(r)
	if cookieSet {
		if id, ok := ctx.TokenManager.Get(token); ok {
			user = id
		}
	}
	if err := ctx.Renderer.RenderAuthentication(w, cookieSet, string(user), string(token)); err != nil {
		RenderError(err)
	}
}

func (ctx *RecipesContext) HandleAuthenticationSet(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	token := r.Form.Get("cookie-input")
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
	ctx.RedirectTo(w, r,"/authentication")
}

func RenderError(err error) {
	logger.Panic("Failed to write string", zap.Error(err))
}

func WriteString(w http.ResponseWriter, s string) {
	_, err := w.Write([]byte(s))
	if err != nil {
		logger.Panic("Failed to write string", zap.Error(err))
	}
}

func (ctx *RecipesContext) HandleHome(w http.ResponseWriter, _ *http.Request) {
	s, err := ctx.Recipes.GetHomePage()
	if err != nil {
		RenderError(err)
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
		RenderError(err)
	}
	WriteString(w, s)
}

func (ctx *RecipesContext) HandleCreate(w http.ResponseWriter, _ *http.Request) {
	s, err := ctx.Recipes.GetCreatePage()
	if err != nil {
		RenderError(err)
	}
	WriteString(w, s)
}

func User(identifier Identifier) zap.Field {
	return zap.String("user", string(identifier))
}

func ReadRecipeRequestResponse(w http.ResponseWriter, r *http.Request, identifier Identifier) (recipe *RawRecipe, rid string, ok bool) {
	recipe, err := ReadRecipeFromResponse(r.Body)
	if err != nil {
		http.Error(w, "invalid-request-body", 400)
		logger.Warn("Failed to read edit post request body", zap.Error(err), User(identifier))
		return nil, "", false
	}

	rid = TransformToIdString(strings.TrimSpace(recipe.Name))
	if len(rid) == 0 {
		http.Error(w, "empty-id", 400)
		logger.Info("Can't create a recipe with an empty id", User(identifier))
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

func ErrorPlaylistAlreadyExists(w http.ResponseWriter, rid string, identifier Identifier) {
	http.Error(w, "duplicate-id", 400)
	logger.Info("A recipe with this id already exists", zap.String("id", rid), User(identifier))
}

func (ctx *RecipesContext) HandleCreateResponse(w http.ResponseWriter, r *http.Request, identifier Identifier) {
	recipe, rid, ok := ReadRecipeRequestResponse(w, r, identifier)
	if !ok {
		return
	}

	alreadyContained, err := ctx.Recipes.AddRecipe(rid, recipe)
	if alreadyContained {
		ErrorPlaylistAlreadyExists(w, rid, identifier)
		return
	}

	if err != nil {
		http.Error(w, "internal-error", 400)
		logger.Warn("Failed to add recipe", zap.String("identifier", rid), User(identifier), zap.Error(err))
		return
	}

	logger.Info("Created recipe", zap.String("identifier", rid), User(identifier), zap.Error(err))
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
		RenderError(err)
	}
	WriteString(w, s)
}

func (ctx *RecipesContext) HandleEditResponse(w http.ResponseWriter, r *http.Request, identifier Identifier) {
	vars := mux.Vars(r)
	oldRid := vars["recipe"]

	recipe, rid, ridOk := ReadRecipeRequestResponse(w, r, identifier)
	if !ridOk {
		return
	}

	alreadyContained, err := ctx.Recipes.ReplaceRecipe(rid, oldRid, recipe)
	if alreadyContained {
		ErrorPlaylistAlreadyExists(w, rid, identifier)
		return
	}
	if err != nil {
		http.Error(w, "internal-error", 400)
		logger.Warn("Failed to replace recipe", zap.String("id", rid), zap.String("old-id", oldRid), User(identifier), zap.Error(err))
		return
	}

	logger.Info("Replaced recipe", zap.String("id", rid), zap.String("old-id", oldRid), User(identifier))
	ctx.RedirectToRecipe(w, r, rid)
}

func (ctx *RecipesContext) HandleDeleteResponse(w http.ResponseWriter, r *http.Request, identifier Identifier) {
	vars := mux.Vars(r)
	rid := vars["recipe"]

	exists, err := ctx.Recipes.RemoveRecipe(rid)
	if !exists {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal-error", 400)
		logger.Warn("Failed to delete recipe", zap.String("id", rid), User(identifier), zap.Error(err))
		return
	}
	logger.Info("Deleted recipe", zap.String("id", rid), User(identifier))
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
	r.HandleFunc("/authentication/set", ctx.HandleAuthenticationSet).Methods("POST").Schemes(scheme)

	r.Handle("/create", ctx.HandleAuthenticate(ctx.HandleCreateResponse)).Methods("POST").Schemes(scheme)
	r.Handle("/delete/{recipe}", ctx.HandleAuthenticate(ctx.HandleDeleteResponse)).Methods("POST").Schemes(scheme)
	r.Handle("/edit/{recipe}", ctx.HandleAuthenticate(ctx.HandleEditResponse)).Methods("POST").Schemes(scheme)

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
	logger.Info("Starting server", zap.String("address", addr), zap.Bool("https", !params.unsecure))
	if params.unsecure {
		return http.ListenAndServe(addr, r)
	} else {
		return http.ListenAndServeTLS(addr, params.certFile, params.keyFile, r)
	}
}

func InitLogger(file string, l zapcore.Level) *zap.Logger {
	logFileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   file,
		MaxSize:    50, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	})
	e := zap.NewProductionEncoderConfig()
	e.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(e),
		zapcore.NewMultiWriteSyncer(logFileWriter, os.Stderr),
		l,
	)
	return zap.New(core)
}

func main() {
	params := ServerParams{}
	logger = InitLogger("logs/server.log", zap.DebugLevel)
	defer func() { _ = logger.Sync() }()

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
				Name:        "tokens-key",
				Value:       "tokens.key",
				Usage:       "tokens key file to use for authorization",
				Destination: &params.tokensKeyPath,
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
		logger.Panic("Fatal error from app", zap.Error(err))
	}
}
