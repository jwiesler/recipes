use std::collections::HashMap;

use actix_web::HttpRequest;
use actix_web::web::{Data, Form, Html, Json, Path, Redirect, ServiceConfig};
use serde::Deserialize;
use serde_json::{json, Value};
use tracing::instrument;

use crate::auth::{Authenticated, WritePermission};
use crate::context::Context;
use crate::error::Error;
use crate::id::to_id_string;
use crate::recipe::RawRecipe;

#[actix_web::get("/")]
async fn page_home(ctx: Data<Context>) -> Html {
    let recipes: HashMap<String, Value> = ctx
        .recipes
        .list()
        .await
        .into_iter()
        .map(|(k, v)| (k, Value::String(v)))
        .collect();
    let context = json!({
        "base_url": "",
        "recipes": recipes
    });

    let content = ctx
        .templates
        .read()
        .await
        .render("home.html", context)
        .unwrap();
    Html::new(content)
}

#[actix_web::get("/login")]
async fn page_login(ctx: Data<Context>) -> Html {
    let context = json!({
        "base_url": "",
    });

    let content = ctx
        .templates
        .read()
        .await
        .render("login.html", context)
        .unwrap();
    Html::new(content)
}

#[actix_web::get("/recipe/{recipe}")]
#[instrument(skip(ctx))]
async fn page_recipe(ctx: Data<Context>, id: Path<String>) -> Result<Html, Error> {
    let id = id.into_inner();
    let recipe = ctx.recipes.get(&id).await?;
    let value = serde_json::to_value(recipe.bake()).unwrap();
    let context = json!({
        "base_url": "",
        "id": id,
        "recipe": value,
    });

    let content = ctx
        .templates
        .read()
        .await
        .render("recipe-page.html", context)
        .unwrap();
    Ok(Html::new(content))
}

#[actix_web::get("/create")]
async fn page_create(ctx: Data<Context>) -> Html {
    let context = json!({
        "base_url": "",
    });
    let content = ctx
        .templates
        .read()
        .await
        .render("edit-recipe-page.html", context)
        .unwrap();
    Html::new(content)
}

#[actix_web::get("/edit/{recipe}")]
#[instrument(skip(ctx))]
async fn page_edit(ctx: Data<Context>, id: Path<String>) -> Result<Html, Error> {
    let id = id.into_inner();
    let recipe = ctx.recipes.get(&id).await?;
    let value = serde_json::to_value(recipe).unwrap();
    let context = json!({
        "base_url": "",
        "id": id,
        "recipe": value,
    });
    let content = ctx
        .templates
        .read()
        .await
        .render("edit-recipe-page.html", context)
        .unwrap();
    Ok(Html::new(content))
}

#[actix_web::post("/create")]
#[instrument(skip(ctx, recipe), fields(name=%recipe.name))]
async fn create(
    ctx: Data<Context>,
    _u: Authenticated<WritePermission>,
    Json(mut recipe): Json<RawRecipe>,
) -> Result<Redirect, Error> {
    recipe.clean();
    let id = to_id_string(&recipe.name);
    if id.is_empty() {
        return Err(Error::EmptyId);
    }
    let url = format!("/recipe/{id}");
    ctx.recipes.create(id, recipe).await?;
    Ok(Redirect::to(url).see_other())
}

#[actix_web::post("/edit/{recipe}")]
#[instrument(skip(ctx, recipe), fields(name=%recipe.name))]
async fn edit(
    ctx: Data<Context>,
    _u: Authenticated<WritePermission>,
    id: Path<String>,
    Json(mut recipe): Json<RawRecipe>,
) -> Result<Redirect, Error> {
    let id = id.into_inner();
    recipe.clean();
    let new_id = to_id_string(&recipe.name);
    if new_id.is_empty() {
        return Err(Error::EmptyId);
    }
    let url = format!("/recipe/{new_id}");
    ctx.recipes.replace(&id, new_id, recipe).await?;
    Ok(Redirect::to(url).see_other())
}

#[actix_web::post("/delete/{recipe}")]
#[instrument(skip(ctx))]
async fn delete(
    ctx: Data<Context>,
    _u: Authenticated<WritePermission>,
    id: Path<String>,
) -> Result<Redirect, Error> {
    let id = id.into_inner();
    ctx.recipes.delete(&id).await?;
    Ok(Redirect::to("/").see_other())
}

#[derive(Deserialize)]
struct LoginForm {
    user: String,
    password: String,
}

#[actix_web::post("/register")]
#[instrument(skip(ctx, password))]
async fn register(
    ctx: Data<Context>,
    Form(LoginForm { user, password }): Form<LoginForm>,
) -> Result<Redirect, Error> {
    ctx.users.register(user, password).await?;
    Ok(Redirect::to("/").see_other())
}

#[actix_web::post("/login")]
#[instrument(skip(ctx, password, req))]
async fn login(
    ctx: Data<Context>,
    Form(LoginForm { user, password }): Form<LoginForm>,
    req: HttpRequest,
) -> Result<Redirect, Error> {
    ctx.users.login(user, password, &req).await?;
    Ok(Redirect::to("/").see_other())
}

#[actix_web::post("/invalidate-sessions")]
#[instrument(skip(ctx, req))]
async fn invalidate_sessions(
    ctx: Data<Context>,
    u: Authenticated<WritePermission>,
    req: HttpRequest,
) -> Result<Redirect, Error> {
    ctx.users.invalidate_sessions(u.0 .0, &req).await?;
    Ok(Redirect::to("/").see_other())
}

pub(crate) fn configure(c: &mut ServiceConfig) {
    c.service(page_home)
        .service(page_login)
        .service(page_recipe)
        .service(page_create)
        .service(page_edit)
        .service(register)
        .service(login)
        .service(invalidate_sessions)
        .service(create)
        .service(edit)
        .service(delete);
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use actix_web::{App, http::header::ContentType, test};
    use actix_web::web::Data;
    use tokio::sync::RwLock;

    use crate::auth::Users;
    use crate::context::Context;
    use crate::recipes::Recipes;
    use crate::templates::Templates;

    use super::configure;

    async fn make_app_data() -> Data<Context> {
        let recipes = Recipes::load_dir(Path::new("tests/recipes")).await;
        let users = Users::load(Path::new("tests/users.json").into()).await;
        let templates = RwLock::new(Templates::load("templates/**/*").await);
        Data::new(Context {
            templates,
            recipes,
            users,
        })
    }

    #[actix_web::test]
    async fn test_home_page() {
        let app = test::init_service(
            App::new()
                .configure(configure)
                .app_data(make_app_data().await),
        )
        .await;
        let req = test::TestRequest::with_uri("/")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }

    #[actix_web::test]
    async fn test_home_login() {
        let app = test::init_service(
            App::new()
                .configure(configure)
                .app_data(make_app_data().await),
        )
        .await;
        let req = test::TestRequest::with_uri("/login")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }

    #[actix_web::test]
    async fn test_create_page() {
        let app = test::init_service(
            App::new()
                .configure(configure)
                .app_data(make_app_data().await),
        )
        .await;
        let req = test::TestRequest::with_uri("/create")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }

    #[actix_web::test]
    async fn test_recipe_page() {
        let app = test::init_service(
            App::new()
                .configure(configure)
                .app_data(make_app_data().await),
        )
        .await;
        let req = test::TestRequest::with_uri("/recipe/test-1")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }

    #[actix_web::test]
    async fn test_edit_page() {
        let app = test::init_service(
            App::new()
                .configure(configure)
                .app_data(make_app_data().await),
        )
        .await;
        let req = test::TestRequest::with_uri("/edit/test-1")
            .insert_header(ContentType::plaintext())
            .to_request();
        let resp = test::call_service(&app, req).await;
        assert!(resp.status().is_success());
    }
}
