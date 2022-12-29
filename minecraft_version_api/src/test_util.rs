use std::fs::File;
use std::io;
use std::path::PathBuf;

pub fn load_test_file(s: &str) -> io::Result<io::BufReader<File>> {
    let mut d = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    d.push("resources/test");
    d.push(s);
    Ok(io::BufReader::new(File::open(d)?))
}

