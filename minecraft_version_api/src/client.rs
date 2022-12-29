use chrono::offset::Utc;
use chrono::DateTime;
use serde::{Deserialize, Serialize};
use url::Url;
use crate::manifest::VersionType;

#[derive(Deserialize, Serialize, Debug)]
#[serde(rename_all = "camelCase")]
pub struct Client {
    pub arguments: Arguments,
    pub id: String,
    pub assets: String,
    pub asset_index: AssetIndex,
    pub compliance_level: u8,
    pub downloads: Downloads,
    pub java_version: JavaVersion,
    pub libraries: Vec<Library>,
    pub logging: Logging,
    pub main_class: String,
    pub minimum_launcher_version: u64,
    #[serde(rename = "type")]
    pub version_type: VersionType,
    pub time: DateTime<Utc>,
    pub release_time: DateTime<Utc>,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct Logging {
    pub client: ClientLogging,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct ClientLogging {
    pub argument: String,
    pub file: LoggingFile,
    #[serde(rename = "type")]
    pub logging_type: String,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct LoggingFile {
    pub id: String, 
    pub sha1: String,
    pub size: u64,
    pub url: Url,
}
#[derive(Deserialize, Serialize, Debug)]
pub struct Library {
    pub name: String,
    pub downloads: LibraryDownloads,
    pub rules: Option<Vec<Rule>>
}

#[derive(Deserialize, Serialize, Debug)]
pub struct LibraryDownloads {
    pub artifact: LibraryArtifact,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct LibraryArtifact {
    pub path: String,
    pub sha1: String,
    pub size: u64,
    pub url: Url,
}

#[derive(Deserialize, Serialize, Debug)]
#[serde(rename_all = "camelCase")]
pub struct JavaVersion {
    component: String,
    major_version: u64,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct Downloads {
    // These keys are snake_case in the JSON, not camelCase.
    client: Download,
    client_mappings: Download,
    server: Download,
    server_mappings: Download,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct Download {
    pub sha1: String,
    pub size: u64,
    pub url: Url,
}

#[derive(Deserialize, Serialize, Debug)]
#[serde(rename_all = "camelCase")]
pub struct AssetIndex {
    pub id: String,
    pub sha1: String,
    pub size: u64,
    pub total_size: u64,
    pub url: Url,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct Arguments {
    pub game: Vec<Argument>,
    pub jvm: Vec<Argument>,
}

#[derive(Deserialize, Serialize, Debug)]
#[serde(untagged)]
pub enum Argument {
    Arg(String),
    Conditional(ConditionalArgument),
}

#[derive(Deserialize, Serialize, Debug)]
pub struct ConditionalArgument {
    pub rules: Vec<Rule>,
    pub value: ConditionalArgumentValue,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct Rule {
    pub action: String,
    pub features: Option<RuleFeatures>,
    pub os: Option<RuleOS>,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct RuleOS {
    pub name: Option<String>,
    pub version: Option<String>,
    pub arch: Option<String>,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct RuleFeatures {
    // Interestingly these fields are *not* camelCase in the JSON. They're
    // snake_case, just like the Rust identifiers. No serde rename required.
    pub is_demo_user: Option<bool>,
    pub has_custom_resolution: Option<bool>,
}

#[derive(Deserialize, Serialize, Debug)]
#[serde(untagged)]
pub enum ConditionalArgumentValue {
    Single(String),
    List(Vec<String>),
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::error::Error;
    use crate::test_util::load_test_file;

    #[test]
    fn parse_1_19_3() -> Result<(), Box<dyn Error>> {
        let c: Client = serde_json::from_reader(load_test_file("1.19.3.json")?)?;
        assert_eq!(c.id, "1.19.3");
        assert_eq!(c.java_version.major_version, 17);
        Ok(())
    }
}

