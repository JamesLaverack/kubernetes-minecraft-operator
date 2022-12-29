use chrono::offset::Utc;
use chrono::DateTime;
use serde::{Deserialize, Serialize};
use url::Url;

#[derive(Deserialize, Serialize, Debug)]
pub struct VersionManifest {
    pub latest: Latest,
    pub versions: Vec<Version>,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct Latest {
    pub release: String,
    pub snapshot: String,
}

#[derive(Deserialize, Serialize, Clone, Debug)]
pub struct Version {
    pub id: String,
    #[serde(rename = "type")]
    pub version_type: VersionType,
    pub url: Url,
    pub time: DateTime<Utc>,
    #[serde(rename = "releaseTime")]
    pub release_time: DateTime<Utc>,
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

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs::File;
    use std::io;
    use std::{error::Error, path::PathBuf};

    fn load_test_file(s: &str) -> io::Result<Box<dyn io::Read>> {
        let mut d = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        d.push("resources/test");
        d.push(s);
        io::BufReader::new(File::open(d)?)
    }

    #[test]
    fn manifest_parse_v1() -> Result<(), Box<dyn Error>> {
        let manifest: VersionManifest = serde_json::from_reader(load_test_file("version_manifest.json"))?;
        assert_eq!(manifest.latest.release, "1.19.3");
        Ok(())
    }

    #[test]
    fn manifest_parse_v2() -> Result<(), Box<dyn Error>> {
        let manifest: VersionManifest = serde_json::from_reader(load_test_file("version_manifest_v2.json"))?;
        assert_eq!(manifest.latest.release, "1.19.3");
        Ok(())
    }

    #[test]
    fn find_exact() -> Result<(), Box<dyn Error>> {
        let manifest: VersionManifest = serde_json::from_reader(load_test_file("version_manifest_v2.json"))?;

        let v = manifest.find_exact("1.18.2");
        assert!(v.is_some());
        let uv = v.unwrap();
        assert_eq!(uv.id, "1.18.2");
        assert_eq!(uv.version_type, VersionType::Release);
        assert_eq!(uv.url.scheme(), "https");
        Ok(())
    }

}

