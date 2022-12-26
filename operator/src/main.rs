use anyhow::Result;
use kube::{
    client::Client,
    runtime::controller::Action,
};
use std::sync::Arc;
use tokio::time::Duration;
use api::MinecraftServer;

#[derive(Clone)]
pub struct Context {
    pub client: Client,
}

async fn reconcile(minecraft: Arc<MinecraftServer>, ctx: Arc<Context>) -> Result<Action> {
    return Ok(Action::requeue(Duration::from_secs(60 * 60)));
}

fn main() {
    println!("Hello, world!");
}
