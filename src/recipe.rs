use std::collections::hash_map::Entry;
use std::collections::HashMap;
use std::str::FromStr;

use comrak::{markdown_to_html, Options};
use serde::{Deserialize, Serialize};

#[derive(Clone, Deserialize, Serialize)]
#[cfg_attr(test, derive(Debug, PartialEq, Eq))]
#[serde(rename_all = "PascalCase")]
pub struct Ingredient {
    pub name: String,
    pub amount: String,
    pub unit: Option<String>,
}

#[derive(Clone, Deserialize, Serialize)]
#[cfg_attr(test, derive(Debug, PartialEq, Eq))]
#[serde(rename_all = "PascalCase")]
pub struct IngredientsSection {
    pub heading: String,
    pub ingredients: Vec<Ingredient>,
}

#[derive(Clone, Deserialize, Serialize)]
#[cfg_attr(test, derive(Debug, PartialEq, Eq))]
#[serde(rename_all = "PascalCase")]
pub struct RawRecipe {
    pub name: String,
    pub description: String,
    pub ingredients_sections: Vec<IngredientsSection>,
    pub instructions: String,
    pub source: String,
}

impl RawRecipe {
    pub fn clean(&mut self) {
        self.name = self.name.trim().to_string();
        self.description = self.description.trim().to_string();
        for s in &mut self.ingredients_sections {
            s.heading = s.heading.trim().to_string();
            for i in &mut s.ingredients {
                i.name = i.name.trim().to_string();
                i.unit = i.unit.as_ref().map(|u| u.trim().to_string());
                i.amount = i.amount.trim().replace(',', ".").to_string();
            }
        }
    }

    pub fn bake(self) -> BakedRecipe {
        let ingredients_sections = self
            .ingredients_sections
            .iter()
            .map(|s| IngredientsSection {
                heading: bake_string(&s.heading),
                ingredients: s
                    .ingredients
                    .iter()
                    .map(|i| Ingredient {
                        name: bake_string(&i.name),
                        amount: bake_string(&i.amount),
                        unit: i.unit.as_deref().map(bake_string),
                    })
                    .collect(),
            })
            .collect::<Vec<_>>();
        BakedRecipe {
            name: bake_string(&self.name),
            description: bake_string(&self.description),
            ingredient_summaries: make_ingredient_summaries(&ingredients_sections),
            ingredients_sections,
            instructions: bake_md_string(&self.instructions),
            source: bake_md_string(&self.source),
        }
    }
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "PascalCase")]
pub struct IngredientSummary {
    name: String,
    unit: Option<String>,
    amount: f64,
    recipe_offset: usize,
}

#[derive(Serialize)]
#[serde(rename_all = "PascalCase")]
pub struct BakedRecipe {
    name: String,
    description: String,
    ingredients_sections: Vec<IngredientsSection>,
    ingredient_summaries: Vec<IngredientSummary>,
    instructions: String,
    source: String,
}

fn bake_md_string(s: &str) -> String {
    let options = Options {
        extension: Default::default(),
        parse: Default::default(),
        render: Default::default(),
    };
    markdown_to_html(s, &options)
}

fn bake_string(s: &str) -> String {
    tera::escape_html(s)
}

fn make_ingredient_summaries(sections: &[IngredientsSection]) -> Vec<IngredientSummary> {
    let mut ingredients: HashMap<_, IngredientSummary> = HashMap::new();

    for section in sections {
        for ingredient in &section.ingredients {
            let Ok(amount) = f64::from_str(&ingredient.amount) else {
                continue;
            };
            let key = (&ingredient.name, &ingredient.unit);
            let len = ingredients.len();
            match ingredients.entry(key) {
                Entry::Occupied(mut e) => {
                    e.get_mut().amount += amount;
                }
                Entry::Vacant(e) => {
                    e.insert(IngredientSummary {
                        name: ingredient.name.clone(),
                        unit: ingredient.unit.clone(),
                        amount,
                        recipe_offset: len,
                    });
                }
            }
        }
    }
    let mut result: Vec<_> = ingredients.into_values().collect();
    result.sort_unstable_by_key(|s| s.recipe_offset);
    result
}
