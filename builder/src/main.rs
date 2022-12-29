use api::MinecraftServer;
use minecraft_version_api::{VERSION_MANIFEST_URL, VersionManifest};
use kube::{
    api::Api,
    client::Client,
};
use std::env;
use std::path::PathBuf;
use reqwest;
extern crate pretty_env_logger;
#[macro_use]
extern crate log;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    pretty_env_logger::init();
    info!("Starting Minecraft server builder");
    let name = env::var("SERVER_NAME")?;
    let namespace = env::var("SERVER_NAMESPACE").unwrap_or("default".to_string());
    info!("Building server {}/{}", name, namespace);

    let tmp_dir = env::var("TMP_DIR").unwrap_or("/tmp".to_string());
    info!("Using {} as temporary directory", tmp_dir);

    let output_dir = env::var("OUTPUT_DIR")?;
    info!("Using {} as the output directory", output_dir);

    debug!("Resolving paths");
    let ctx = Context::new(tmp_dir, output_dir)?;

    debug!("Contacting Kubernetes API");
    let client = Client::try_default().await?;
    let mc_client = Api::<MinecraftServer>::namespaced(client.clone(), namespace.as_str());

    debug!("Trying to find our server object");
    let server = mc_client.get(name.as_str()).await?;

    download_server(ctx, server.spec).await?;

    Ok(())
}

struct Context {
    tmp_dir: PathBuf,
    output_dir: PathBuf,
}

impl Context {
    fn new(tmp_dir: String, output_dir: String) -> anyhow::Result<Context> {
        let tp = PathBuf::from(tmp_dir);
        let op = PathBuf::from(output_dir);
        if !tp.is_dir() {
            anyhow::bail!("Temp dir does not exist")
        }
        if !op.is_dir() {
            anyhow::bail!("Output dir does not exist")
        }
        Ok(Context {
            tmp_dir: tp,
            output_dir: op,
        })
    }
}

async fn download_server(context: Context, spec: api::MinecraftServerSpec) -> anyhow::Result<()> {
    match spec.server_type {
        api::ServerType::Vanilla => {
            info!("Downloading Vanilla server JAR from Mojang");
            let u = reqwest::get(VERSION_MANIFEST_URL)
                .await?
                .json::<VersionManifest>()
                .await?
                .find_exact(&spec.version.minecraft)
                .unwrap()
                .url;
            info!("Minecraft URL is {:?}", u)
        }
        api::ServerType::Paper => {
            info!("Paper")
        }
        api::ServerType::Forge => {
            info!("Forge")
        }
    }
    Ok(())
}

