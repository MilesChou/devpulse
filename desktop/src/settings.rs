//! Where the dashboard keeps its connection settings.
//!
//! The server URL is not secret and lives in a JSON file under the OS
//! config directory. The API token is a credential, so it goes to the
//! OS keychain (macOS Keychain, Windows Credential Manager, Secret
//! Service on Linux) and never touches the file.

use std::fs;
use std::io;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use crate::i18n::Lang;

pub const DEFAULT_BASE_URL: &str = "http://127.0.0.1:8080";

/// Non-secret settings persisted as JSON.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct Settings {
    pub base_url: String,
    /// Last selected repo (`owner/name`), restored on the next launch.
    pub last_repo: Option<String>,
    /// UI language chosen in Settings. `None` until the user picks one,
    /// so the OS locale decides. An unknown value (say, from a newer
    /// build) also reads as `None` instead of discarding the whole file.
    #[serde(deserialize_with = "lenient_lang")]
    pub language: Option<Lang>,
}

impl Default for Settings {
    fn default() -> Self {
        Self {
            base_url: DEFAULT_BASE_URL.to_string(),
            last_repo: None,
            language: None,
        }
    }
}

fn lenient_lang<'de, D: serde::Deserializer<'de>>(d: D) -> Result<Option<Lang>, D::Error> {
    let raw = Option::<serde_json::Value>::deserialize(d)?;
    Ok(raw.and_then(|v| serde_json::from_value(v).ok()))
}

/// Default location: `<config dir>/devpulse/desktop.json`. The
/// `DEVPULSE_DESKTOP_CONFIG` environment variable overrides it (see
/// `main`).
pub fn default_path() -> Option<PathBuf> {
    dirs::config_dir().map(|d| d.join("devpulse").join("desktop.json"))
}

/// Loads settings, falling back to defaults when the file is missing.
/// A corrupt file also falls back to defaults: the file only holds a
/// URL, a selection and a language, so losing it costs one re-entry.
pub fn load(path: &Path) -> Settings {
    fs::read_to_string(path)
        .ok()
        .and_then(|s| serde_json::from_str(&s).ok())
        .unwrap_or_default()
}

pub fn save(path: &Path, settings: &Settings) -> io::Result<()> {
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir)?;
    }
    let json = serde_json::to_string_pretty(settings).map_err(io::Error::other)?;
    fs::write(path, json + "\n")
}

/// Decides which token is in use after the user saves the connection
/// form. A typed token always wins. With the field left empty, the
/// current token stays as long as the server URL is unchanged, even if
/// it was never stored (it came from `DEVPULSE_API_TOKEN`, or the
/// keychain write failed); only a new URL looks up that server's stored
/// token.
pub fn token_after_save(
    current: Option<String>,
    url_changed: bool,
    typed: &str,
    stored: impl FnOnce() -> Option<String>,
) -> Option<String> {
    let typed = typed.trim();
    if !typed.is_empty() {
        return Some(typed.to_string());
    }
    if !url_changed && current.is_some() {
        return current;
    }
    stored()
}

/// Storage for the API token.
pub trait SecretStore: Send {
    /// Returns the stored token, or `None` when none has been saved.
    fn get(&self) -> Result<Option<String>, String>;
    fn set(&mut self, token: &str) -> Result<(), String>;
    fn delete(&mut self) -> Result<(), String>;
}

/// Keychain-backed store: one entry per server URL, so switching
/// between servers does not overwrite another server's token.
pub struct KeyringStore {
    service: String,
    account: String,
}

impl KeyringStore {
    const SERVICE: &'static str = "devpulse-desktop";

    pub fn for_server(base_url: &str) -> Self {
        Self {
            service: Self::SERVICE.to_string(),
            account: base_url.trim().trim_end_matches('/').to_string(),
        }
    }

    fn entry(&self) -> Result<keyring::Entry, String> {
        keyring::Entry::new(&self.service, &self.account).map_err(|e| e.to_string())
    }
}

impl SecretStore for KeyringStore {
    fn get(&self) -> Result<Option<String>, String> {
        match self.entry()?.get_password() {
            Ok(token) => Ok(Some(token)),
            Err(keyring::Error::NoEntry) => Ok(None),
            Err(e) => Err(e.to_string()),
        }
    }

    fn set(&mut self, token: &str) -> Result<(), String> {
        self.entry()?.set_password(token).map_err(|e| e.to_string())
    }

    fn delete(&mut self) -> Result<(), String> {
        match self.entry()?.delete_credential() {
            Ok(()) | Err(keyring::Error::NoEntry) => Ok(()),
            Err(e) => Err(e.to_string()),
        }
    }
}

/// In-process store for tests.
#[cfg(test)]
#[derive(Default)]
pub struct MemoryStore(Option<String>);

#[cfg(test)]
impl SecretStore for MemoryStore {
    fn get(&self) -> Result<Option<String>, String> {
        Ok(self.0.clone())
    }

    fn set(&mut self, token: &str) -> Result<(), String> {
        self.0 = Some(token.to_string());
        Ok(())
    }

    fn delete(&mut self) -> Result<(), String> {
        self.0 = None;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn temp_path(name: &str) -> PathBuf {
        let dir = std::env::temp_dir().join(format!(
            "devpulse-desktop-test-{}-{name}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&dir);
        dir.join("nested").join("desktop.json")
    }

    #[test]
    fn missing_file_yields_defaults() {
        let path = temp_path("missing");
        assert_eq!(load(&path), Settings::default());
    }

    #[test]
    fn round_trips_and_creates_directories() {
        let path = temp_path("roundtrip");
        let s = Settings {
            base_url: "https://devpulse.example.com".into(),
            last_repo: Some("MilesChou/devpulse".into()),
            language: Some(Lang::ZhTw),
        };
        save(&path, &s).expect("save");
        assert_eq!(load(&path), s);
        assert!(
            !fs::read_to_string(&path).unwrap().contains("token"),
            "the settings file must never hold the token"
        );
    }

    #[test]
    fn corrupt_or_partial_file_falls_back() {
        let path = temp_path("corrupt");
        fs::create_dir_all(path.parent().unwrap()).unwrap();
        fs::write(&path, "{not json").unwrap();
        assert_eq!(load(&path), Settings::default());

        // Unknown keys are ignored and missing keys take defaults.
        fs::write(&path, r#"{"last_repo":"a/b","extra":1}"#).unwrap();
        let s = load(&path);
        assert_eq!(s.base_url, DEFAULT_BASE_URL);
        assert_eq!(s.last_repo.as_deref(), Some("a/b"));
        // Files written before the language setting follow the locale.
        assert_eq!(s.language, None);

        // An unknown language keeps the rest of the file.
        fs::write(&path, r#"{"last_repo":"a/b","language":"fr"}"#).unwrap();
        let s = load(&path);
        assert_eq!(s.last_repo.as_deref(), Some("a/b"));
        assert_eq!(s.language, None);
    }

    #[test]
    fn language_is_stored_by_tag() {
        let path = temp_path("language");
        let s = Settings {
            language: Some(Lang::ZhTw),
            ..Settings::default()
        };
        save(&path, &s).expect("save");
        assert!(
            fs::read_to_string(&path)
                .unwrap()
                .contains(r#""language": "zh-TW""#)
        );
        assert_eq!(load(&path).language, Some(Lang::ZhTw));
    }

    #[test]
    fn memory_store_behaves_like_a_keychain() {
        let mut store = MemoryStore::default();
        assert_eq!(store.get(), Ok(None));
        store.set("tok").unwrap();
        assert_eq!(store.get(), Ok(Some("tok".into())));
        store.delete().unwrap();
        store.delete().unwrap(); // deleting twice is fine
        assert_eq!(store.get(), Ok(None));
    }

    #[test]
    fn token_after_save_rules() {
        let cur = || Some("session".to_string());
        let stored = || Some("stored".to_string());
        let none = || None;

        // A typed token always wins.
        assert_eq!(
            token_after_save(cur(), false, " typed ", stored).as_deref(),
            Some("typed")
        );
        assert_eq!(
            token_after_save(None, true, "typed", none).as_deref(),
            Some("typed")
        );

        // Empty field, same server: keep the current (maybe session-only)
        // token without consulting the keychain.
        assert_eq!(
            token_after_save(cur(), false, "", || panic!("must not look up")).as_deref(),
            Some("session")
        );

        // Empty field, same server, nothing in use yet: try the keychain.
        assert_eq!(
            token_after_save(None, false, "", stored).as_deref(),
            Some("stored")
        );

        // Empty field, new server: that server's stored token, or none.
        assert_eq!(
            token_after_save(cur(), true, "  ", stored).as_deref(),
            Some("stored")
        );
        assert_eq!(token_after_save(cur(), true, "", none), None);
    }

    #[test]
    fn keyring_account_ignores_trailing_slash() {
        let a = KeyringStore::for_server("http://host:8080/");
        let b = KeyringStore::for_server(" http://host:8080 ");
        assert_eq!(a.account, b.account);
    }
}
