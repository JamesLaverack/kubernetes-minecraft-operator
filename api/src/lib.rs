use kube::{CustomResource};
use serde::{Deserialize, Serialize};
use schemars::JsonSchema;

#[derive(CustomResource, Deserialize, Serialize, Clone, Debug, JsonSchema)]
#[cfg_attr(test, derive(Default))]
#[kube(kind = "MinecraftServer", group = "minecraft.laverack.dev", version = "v1alpha2", namespaced)]
#[kube(status = "MinecraftServerStatus", shortname = "mcsrv")]
#[serde(rename_all = "camelCase")]
pub struct MinecraftServerSpec {
    pub version: String,
    pub java_major_version: String,
}

#[derive(Deserialize, Serialize, Clone, Default, Debug, JsonSchema)]
pub struct MinecraftServerStatus {
    pub java_version: String,
}

