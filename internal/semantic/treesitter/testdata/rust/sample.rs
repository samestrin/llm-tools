use std::collections::HashMap;

pub struct Config {
    pub api_url: String,
    pub timeout: u64,
}

enum Status {
    Active,
    Inactive,
    Pending(String),
}

trait Repository {
    fn find(&self, id: u64) -> Option<Config>;
    fn save(&mut self, config: Config) -> Result<(), String>;
}

impl Config {
    pub fn new(url: String) -> Self {
        Config {
            api_url: url,
            timeout: 30,
        }
    }

    pub fn with_timeout(mut self, timeout: u64) -> Self {
        self.timeout = timeout;
        self
    }
}

impl Repository for HashMap<u64, Config> {
    fn find(&self, id: u64) -> Option<Config> {
        self.get(&id).cloned()
    }

    fn save(&mut self, config: Config) -> Result<(), String> {
        self.insert(0, config);
        Ok(())
    }
}

pub fn process_data(items: &[Config]) -> Vec<String> {
    items.iter().map(|c| c.api_url.clone()).collect()
}

type ConfigMap = HashMap<String, Config>;
