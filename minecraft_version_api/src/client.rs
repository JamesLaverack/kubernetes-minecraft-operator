use chrono::offset::Utc;
use chrono::DateTime;
use serde::{Deserialize, Serialize};
use url::Url;

#[derive(Deserialize, Serialize, Debug)]
pub struct Client {
    pub arguments: Arguments,
    pub id: String,
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
    pub rules: Vec<ConditionalArgumentRule>,
    pub value: ConditionalArgumentValue,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct ConditionalArgumentRule {
    pub action: String,
    pub features: Option<ConditionalArgumentRuleFeatures>,
    pub os: Option<ConditionalAargumentRuleOS>,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct ConditionalAargumentRuleOS {
    pub name: Option<String>,
    pub version: Option<String>,
    pub arch: Option<String>,
}

#[derive(Deserialize, Serialize, Debug)]
pub struct ConditionalArgumentRuleFeatures {
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
        Ok(())
    }
}

