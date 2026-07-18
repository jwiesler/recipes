use std::path::PathBuf;

use tera::{Context, Kwargs, State, Tera};
use tokio::task::spawn_blocking;

use crate::unit::unit_needs_space;

pub struct Templates(Tera);

impl Templates {
    pub async fn load_directory(dir: PathBuf) -> Self {
        spawn_blocking(move || {
            let files = std::fs::read_dir(&dir)
                .unwrap_or_else(|e| panic!("Failed to read dir {}: {e}", dir.display()))
                .map(|entry| {
                    let path = entry
                        .unwrap_or_else(|e| panic!("Failed to read dir {}: {e}", dir.display()))
                        .path();
                    let name = path.file_name().unwrap().to_str().unwrap().to_owned();
                    (path, Some(name))
                });

            let mut tera = Tera::new();
            tera.register_test(
                "whiteSpacedUnit",
                |value: tera::Value, _: Kwargs, _: &State<'_>| {
                    if let Some(value) = value.as_str() {
                        Ok(unit_needs_space(value))
                    } else if value.is_none() {
                        Ok(false)
                    } else {
                        Err(tera::Error::message(
                            "value is not a string or too many arguments were given",
                        ))
                    }
                },
            );
            tera.add_template_files(files)
                .unwrap_or_else(|e| panic!("Failed to parse template: {e}"));
            tera.autoescape_on(std::iter::empty::<&str>());
            Templates(tera)
        })
        .await
        .unwrap()
    }

    pub fn render(&self, name: &str, context: &Context) -> String {
        self.0
            .render(name, context)
            .unwrap_or_else(|e| panic!("Failed to render template {name}: {e}"))
    }
}
