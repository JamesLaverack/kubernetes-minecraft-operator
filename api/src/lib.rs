use k8s_openapi::api::core::v1::ServiceSpec;
use kube::{CustomResource, core::ObjectMeta};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(CustomResource, Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
#[kube(
    kind = "MinecraftServer",
    group = "minecraft.laverack.dev",
    version = "v1alpha2",
    namespaced
)]
#[kube(status = "MinecraftServerStatus", shortname = "mcsrv")]
#[serde(rename_all = "camelCase")]
pub struct MinecraftServerSpec {
    pub eula: EULA,
    pub version: MinecraftVersion,
    pub server_type: ServerType,
    pub game: GameSpec,
    pub motd: Option<String>,
    pub players: PlayersSpec,
    pub world: WorldSpec,
    pub service_template: ServiceTemplateSpec,
    pub datapacks: Vec<Datapack>,
    pub mods: Vec<Mod>,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct GameSpec {
    pub game_mode: GameMode,
}


#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct ServiceTemplateSpec {
    pub metadata: Option<ObjectMeta>,
    pub spec: ServiceSpec,
}

// A "Mod" can describe either a single mod or a modpack, which is inferred from context.
#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct Mod {
    pub file: Option<File>,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct Datapack {
    pub vanilla_tweaks: Option<Vec<VanillaTweak>>,
    pub file: Option<File>,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct File {
    pub url: String,
    pub checksum: Option<Checksum>,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct VanillaTweak {
    pub category: String,
    pub name: String,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct WorldSpec {
    pub seed: Option<String>,
    pub claim_name: String, 
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct PlayersSpec {
    pub maximum_online_players: Option<u32>,
    pub access_mode: AccessMode,
    pub allow_list: Vec<Player>,
    pub operator_list: Vec<Player>,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct Player {
    pub name: Option<String>,
    pub uuid: Option<String>,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub enum EULA {
    #[default]
    NotAccepted,
    Accepted,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub enum GameMode {
    #[default]
    Survival,
    Creative,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub enum AccessMode {
    #[default]
    AllowListOnly,
    Public,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub enum ServerType {
    #[default]
    Vanilla,
    Paper,
    Forge,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
#[serde(rename_all = "camelCase")]
pub struct MinecraftVersion {
    pub minecraft: String,
    pub java: Option<String>,
    pub forge: Option<ForgeVersion>,
    pub paper: Option<PaperVersion>,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
#[serde(rename_all = "camelCase")]
pub struct PaperVersion {
    pub build: String,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
#[serde(rename_all = "camelCase")]
pub struct ForgeVersion {
    pub version: String,

    // As far as I am aware there's no API to get forge checksums, so we let the
    // user provide it if they want extra security.
    pub installer_checksum: Option<Checksum>,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
#[serde(rename_all = "camelCase")]
pub struct Checksum {
    pub md5: Option<String>,
    pub sha1: Option<String>,
    pub sha256: Option<String>,
}

#[derive(Deserialize, Serialize, Clone, Default, Debug, JsonSchema)]
pub struct MinecraftServerStatus {
    pub java_version: String,
}
