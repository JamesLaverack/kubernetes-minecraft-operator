use minecraft_version_api::{
    client::Client,
    manifest::{VersionManifest, VERSION_MANIFEST_URL},
};
use reqwest;

#[tokio::test]
async fn test_with_network() -> anyhow::Result<()> {
    let manifest = reqwest::get(VERSION_MANIFEST_URL)
        .await?
        .json::<VersionManifest>()
        .await?;
    let version = reqwest::get(manifest.find_exact("1.18.2").unwrap().url)
        .await?
        .json::<Client>()
        .await?;
    assert_eq!(17, version.java_version.unwrap().major_version);
    Ok(())
}

#[tokio::test]
async fn test_all_versions_with_network() -> anyhow::Result<()> {
    let manifest = reqwest::get(VERSION_MANIFEST_URL)
        .await?
        .json::<VersionManifest>()
        .await?;
    for version in &manifest.versions {
        println!("Version {:?}, url {:?}", version.id, version.url);
        let version_client = reqwest::get(version.url.clone()).await?.json::<Client>().await?;
        assert_eq!(version.id, version_client.id);
    }
    Ok(())
}
