use api::MinecraftServer;
use kube::{client::Client, runtime::controller::{Action, Controller}, api::{Api, ListParams}};
use std::sync::Arc;
use tokio::time::Duration;
use thiserror::Error;
use futures::{future::BoxFuture, FutureExt, StreamExt};
extern crate pretty_env_logger;
#[macro_use] extern crate log;

#[derive(Clone)]
pub struct Context {
    pub client: Client,
}

async fn reconcile(server: Arc<MinecraftServer>, ctx: Arc<Context>) -> Result<Action> {
    info!("Reconciling {}", server.metadata.name.as_ref().unwrap());
    return Ok(Action::requeue(Duration::from_secs(60 * 60)));
}

#[derive(Error, Debug)]
pub enum Error {
    #[error("Kube Error: {0}")]
    KubeError(#[source] kube::Error),
}
pub type Result<T, E = Error> = std::result::Result<T, E>;

fn error_policy(server: Arc<MinecraftServer>, error: &Error, ctx: Arc<Context>) -> Action {
    Action::requeue(Duration::from_secs(5 * 60))
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    pretty_env_logger::init();
    info!("Launching kubernetes-minecraft-operator operator");
    let client = Client::try_default().await?;

    let mc_client = Api::<MinecraftServer>::all(client.clone());

    Controller::new(mc_client, ListParams::default())
        //.shutdown_on_signal()
        .run(reconcile, error_policy, Arc::new(Context {client}))
        .for_each(|res| async move {
            match res {
                Ok(o) => info!("reconciled {:?}", o),
                Err(e) => warn!("reconcile failed: {}", e),
            }
        })
        .await;
    Ok(())
}

