use serde_json::Value;
use tera::{Context, Tera};
use tokio::task::spawn_blocking;

use crate::unit::unit_needs_space;

pub struct Templates(Tera);

impl Templates {
    pub async fn load(pattern: &'static str) -> Self {
        spawn_blocking(|| {
            let mut tera =
                Tera::new(pattern).unwrap_or_else(|e| panic!("Failed to parse template: {e}"));
            tera.register_tester(
                "whiteSpacedUnit",
                |value: Option<&Value>, args: &[Value]| match value {
                    Some(Value::String(value)) if args.is_empty() => Ok(unit_needs_space(value)),
                    None | Some(Value::Null) if args.is_empty() => Ok(false),
                    _ => Err(tera::Error::msg(
                        "value is not a string or too many arguments were given",
                    )),
                },
            );
            tera.autoescape_on(vec![]);
            Templates(tera)
        })
        .await
        .unwrap()
    }

    pub fn render(&self, name: &str, context: Value) -> tera::Result<String> {
        let context = Context::from_value(context)?;
        self.0.render(name, &context)
    }
}
