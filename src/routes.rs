use std::collections::HashMap;

use actix_web::web::{Data, Html, Json, Path, Redirect, ServiceConfig};
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
        .into_iter()
        .map(|(k, v)| (k, Value::String(v)))
        .collect();
    let context = json!({
        "base_url": "",
        "recipes": recipes
    });

    let content = ctx.templates.render("home.html", context).unwrap();
    Html::new(content)
}

#[actix_web::get("/recipe/{recipe}")]
#[instrument(skip(ctx))]
async fn page_recipe(ctx: Data<Context>, id: Path<String>) -> Result<Html, Error> {
    let id = id.into_inner();
    let recipe = ctx.recipes.get(&id)?;
    let value = serde_json::to_value(recipe.bake()).unwrap();
    let context = json!({
        "base_url": "",
        "id": id,
        "recipe": value,
    });

    let content = ctx.templates.render("recipe-page.html", context).unwrap();
    Ok(Html::new(content))
}

#[actix_web::get("/create")]
async fn page_create(ctx: Data<Context>) -> Html {
    let context = json!({
        "base_url": "",
    });
    let content = ctx
        .templates
        .render("edit-recipe-page.html", context)
        .unwrap();
    Html::new(content)
}

#[actix_web::get("/edit/{recipe}")]
#[instrument(skip(ctx))]
async fn page_edit(ctx: Data<Context>, id: Path<String>) -> Result<Html, Error> {
    let id = id.into_inner();
    let recipe = ctx.recipes.get(&id)?;
    let value = serde_json::to_value(recipe).unwrap();
    let context = json!({
        "base_url": "",
        "id": id,
        "recipe": value,
    });
    let content = ctx
        .templates
        .render("edit-recipe-page.html", context)
        .unwrap();
    Ok(Html::new(content))
}

#[actix_web::post("/create")]
#[instrument(skip(ctx, recipe), fields(name=%recipe.name))]
async fn create(
    ctx: Data<Context>,
    _: Authenticated<WritePermission>,
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
    _: Authenticated<WritePermission>,
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
    _: Authenticated<WritePermission>,
    id: Path<String>,
) -> Result<Redirect, Error> {
    let id = id.into_inner();
    ctx.recipes.delete(&id).await?;
    Ok(Redirect::to("/").see_other())
}

pub(crate) fn configure(c: &mut ServiceConfig) {
    c.service(page_home)
        .service(page_recipe)
        .service(page_create)
        .service(page_edit)
        .service(create)
        .service(edit)
        .service(delete);
}

#[cfg(test)]
mod tests {
    use actix_web::{App, http::header::ContentType, test};

    use crate::recipes::Recipes;
    use crate::templates::Templates;

    use super::*;

    async fn make_app_data() -> Data<Context> {
        let recipes = Recipes::load_dir(std::path::Path::new("tests/recipes")).await;
        let templates = Templates::load("templates/**/*").await;
        Data::new(Context { templates, recipes })
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
