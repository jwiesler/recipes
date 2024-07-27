use tokio::sync::RwLock;

use crate::auth::Users;
use crate::recipes::Recipes;
use crate::templates::Templates;

pub struct Context {
    pub templates: RwLock<Templates>,
    pub recipes: Recipes,
    pub users: Users,
}
