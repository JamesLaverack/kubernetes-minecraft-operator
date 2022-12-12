use kube::CustomResourceExt;
fn main() {
    print!("{}", serde_yaml::to_string(&api::MinecraftServer::crd()).unwrap())
}
