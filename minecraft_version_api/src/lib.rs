use chrono::offset::Utc;
use chrono::DateTime;
use serde::{Deserialize, Serialize};
use url::Url;

#[derive(Deserialize, Serialize, Debug)]
pub struct VersionManifest {
    latest: Latest,
    versions: Vec<Version>,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct Latest {
    release: String,
    snapshot: String,
}

#[derive(Deserialize, Serialize, Clone, Debug)]
pub struct Version {
    id: String,
    #[serde(rename = "type")]
    version_type: VersionType,
    url: Url,
    time: DateTime<Utc>,
    #[serde(rename = "releaseTime")]
    release_time: DateTime<Utc>,
}

#[derive(Deserialize, Serialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum VersionType {
    Release,
    Snapshot,
    OldAlpha,
    OldBeta,
}

pub const VERSION_MANIFEST_URL: &str =
    "https://launchermeta.mojang.com/mc/game/version_manifest.json";

impl VersionManifest {
    pub fn find_exact(&self, id: &str) -> Option<Version> {
        for version in &self.versions {
            if version.id == id {
                return Some(version.clone());
            }
        }
        return None;
    }

    pub fn latest_release(&self) -> Option<Version> {
        return self.find_exact(&self.latest.release);
    }

    pub fn latest_snapshot(&self) -> Option<Version> {
        return self.find_exact(&self.latest.snapshot);
    }
}

fn is_latest_release(version: &str) -> bool {
    return version == "latest"
        || version == "release"
        || version == "latest_release"
        || version == "latest-release"
        || version == "LatestRelease"
        || version == "latestRelease";
}

fn is_latest_snapshot(version: &str) -> bool {
    return version == "snapshot"
        || version == "latest_snapshot"
        || version == "latest-snapshot"
        || version == "LatestSnapshot"
        || version == "latestSnapshot";
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs::File;
    use std::io::BufReader;
    use std::{error::Error, path::PathBuf};

    #[test]
    fn manifest_parse() -> Result<(), Box<dyn Error>> {
        let mut d = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        d.push("resources/test/version_manifest.json");
        let file = File::open(d)?;
        let reader = BufReader::new(file);
        let manifest: VersionManifest = serde_json::from_reader(reader)?;

        assert_eq!(manifest.latest.release, "1.19.3");
        Ok(())
    }

    #[test]
    fn find_exact() -> Result<(), Box<dyn Error>> {
        let mut d = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        d.push("resources/test/version_manifest.json");
        let file = File::open(d)?;
        let reader = BufReader::new(file);
        let manifest: VersionManifest = serde_json::from_reader(reader)?;

        let v = manifest.find_exact("1.18.2");
        assert!(v.is_some());
        let uv = v.unwrap();
        assert_eq!(uv.id, "1.18.2");
        assert_eq!(uv.version_type, VersionType::Release);
        assert_eq!(uv.url.scheme(), "https");
        Ok(())
    }

}

