use std::collections::HashMap;
use std::collections::hash_map::Entry;
use std::path::{Path, PathBuf};

use tokio::fs::{read_dir, read_to_string};
use tokio::sync::{Mutex, RwLock, RwLockReadGuard};
use tracing::{error, warn};

use crate::error::Error;
use crate::id::to_id_string;
use crate::recipe::RawRecipe;

pub fn handle_io_error(path: &Path, e: &std::io::Error) -> Error {
    error!("Failed to write {path:?}: {e}");
    Error::Internal
}

struct RecipesIo(PathBuf);

struct Write {
    path: PathBuf,
    content: String,
}

struct Delete {
    path: PathBuf,
}

impl RecipesIo {
    async fn read(path: &Path) -> std::io::Result<RawRecipe> {
        let text = read_to_string(path).await?;
        let recipe = serde_json::from_str(&text).unwrap();
        Ok(recipe)
    }

    fn path_of(&self, id: &str) -> PathBuf {
        let mut path = self.0.join(id);
        path.set_extension("json");
        path
    }

    fn prepare_write(&self, id: &str, recipe: &RawRecipe) -> Write {
        let path = self.path_of(id);
        let content = serde_json::to_string(&recipe).unwrap();
        Write { path, content }
    }

    fn prepare_delete(&self, id: &str) -> Delete {
        let path = self.path_of(id);
        Delete { path }
    }

    async fn write(&mut self, write: &Write) -> Result<(), Error> {
        tokio::fs::write(&write.path, &write.content)
            .await
            .map_err(|e| handle_io_error(&write.path, &e))
    }

    async fn delete(&mut self, delete: &Delete) -> Result<(), Error> {
        tokio::fs::remove_file(&delete.path)
            .await
            .map_err(|e| handle_io_error(&delete.path, &e))
    }
}

pub struct Recipes {
    recipes: RwLock<HashMap<String, RawRecipe>>,
    io: Mutex<RecipesIo>,
}

impl Recipes {
    pub async fn load_dir(path: &Path) -> Recipes {
        let mut t = read_dir(path)
            .await
            .unwrap_or_else(|e| panic!("Failed to list recipes dir: {e}"));
        let mut recipes = HashMap::new();
        while let Some(t) = t
            .next_entry()
            .await
            .unwrap_or_else(|e| panic!("Failed to list recipes dir: {e}"))
        {
            let path = t.path();
            let recipe = RecipesIo::read(&path)
                .await
                .unwrap_or_else(|e| panic!("Failed to read {}: {e}", path.display()));
            let id = to_id_string(&recipe.name);
            if id != path.file_stem().unwrap().to_str().unwrap() {
                let file_name = path.file_name().unwrap().to_str().unwrap();
                warn!(
                    "Recipe {:?} has a name {:?} that does not fit its id {id:?}",
                    recipe.name, file_name
                );
            }
            recipes.insert(id, recipe);
        }
        Recipes {
            recipes: RwLock::new(recipes),
            io: Mutex::new(RecipesIo(path.to_path_buf())),
        }
    }

    pub async fn list(&self) -> RwLockReadGuard<'_, HashMap<String, RawRecipe>> {
        self.recipes.read().await
    }

    pub async fn get(&self, id: &str) -> Result<RawRecipe, Error> {
        let recipes = self.recipes.read().await;
        recipes.get(id).cloned().ok_or(Error::NotFound)
    }

    pub async fn create(&self, id: String, recipe: RawRecipe) -> Result<(), Error> {
        let mut io = self.io.lock().await;
        let write = io.prepare_write(&id, &recipe);
        let mut recipes = self.recipes.write().await;
        match recipes.entry(id) {
            Entry::Occupied(_) => {
                return Err(Error::AlreadyExists);
            }
            Entry::Vacant(e) => {
                e.insert(recipe);
                drop(recipes);
                io.write(&write).await?;
            }
        }
        Ok(())
    }

    pub async fn delete(&self, id: &str) -> Result<(), Error> {
        let mut io = self.io.lock().await;
        let delete = io.prepare_delete(id);
        let mut recipes = self.recipes.write().await;
        recipes.remove(id).ok_or(Error::NotFound)?;
        io.delete(&delete).await
    }

    pub async fn replace(&self, id: &str, new_id: String, recipe: RawRecipe) -> Result<(), Error> {
        let mut io = self.io.lock().await;
        let write = io.prepare_write(&new_id, &recipe);
        let delete = io.prepare_delete(id);
        let mut recipes = self.recipes.write().await;
        match recipes.entry(new_id) {
            Entry::Occupied(mut e) => {
                if id == e.key() {
                    e.insert(recipe);
                    drop(recipes);
                    io.write(&write).await?;
                } else {
                    return Err(Error::AlreadyExists);
                }
            }
            Entry::Vacant(e) => {
                e.insert(recipe);
                recipes.remove(id);
                drop(recipes);
                io.write(&write).await?;
                io.delete(&delete).await?;
            }
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use tempfile::TempDir;

    use crate::recipe::{Ingredient, IngredientsSection, RawRecipe};
    use crate::recipes::Recipes;

    #[tokio::test]
    async fn test_read_write() {
        let dir = TempDir::new().unwrap();
        let path = dir.path();
        let recipes = Recipes::load_dir(path).await;

        assert_eq!(&*recipes.list().await, &HashMap::new());

        let recipe = RawRecipe {
            name: "test 1".to_string(),
            description: "b".to_string(),
            ingredients_sections: vec![IngredientsSection {
                heading: "f".to_string(),
                ingredients: vec![Ingredient {
                    name: "g".to_string(),
                    amount: "h".to_string(),
                    unit: Some("i".to_string()),
                }],
            }],
            instructions: "d".to_string(),
            source: "e".to_string(),
            categories: vec![],
        };
        recipes
            .create("test-1".to_string(), recipe.clone())
            .await
            .unwrap();
        assert_eq!(
            &recipes
                .list()
                .await
                .iter()
                .map(|(k, v)| (k.as_str(), v.name.as_str()))
                .collect::<Vec<_>>(),
            &[("test-1", "test 1")]
        );
        assert_eq!(recipes.recipes.read().await.get("test-1"), Some(&recipe));

        {
            let recipes = Recipes::load_dir(path).await;
            assert_eq!(
                &recipes
                    .list()
                    .await
                    .iter()
                    .map(|(k, v)| (k.as_str(), v.name.as_str()))
                    .collect::<Vec<_>>(),
                &[("test-1", "test 1")]
            );
        }

        recipes.delete("test-1").await.unwrap();

        {
            let recipes = Recipes::load_dir(path).await;
            assert_eq!(&*recipes.list().await, &HashMap::new());
        }
    }
}
