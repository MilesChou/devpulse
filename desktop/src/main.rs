//! DevPulse desktop dashboard.
//!
//! A read-only client of the JSON API that `devpulse serve` exposes: it
//! lists tracked repos and charts their CI and PR metrics. All data
//! collection (GitHub, CI providers, the database) stays in the Go
//! service; this app only needs the server URL and its API token.
//! The UI is in English or Traditional Chinese (see `i18n`).

mod api;
mod app;
mod fonts;
mod i18n;
mod kpi;
mod month;
mod settings;
mod state;

use eframe::egui;

use crate::app::{DashboardApp, Overrides};
use crate::settings::{KeyringStore, SecretStore};

fn main() -> eframe::Result {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_title("DevPulse")
            .with_inner_size([1200.0, 820.0])
            .with_min_inner_size([800.0, 560.0]),
        ..Default::default()
    };

    eframe::run_native(
        "DevPulse",
        options,
        Box::new(|cc| {
            let missing_cjk_font = match fonts::find_cjk() {
                Some(font) => {
                    fonts::install(&cc.egui_ctx, font);
                    None
                }
                None => Some(fonts::candidates()),
            };
            let make_store = Box::new(|url: &str| -> Box<dyn SecretStore> {
                Box::new(KeyringStore::for_server(url))
            });
            Ok(Box::new(DashboardApp::new(
                cc.egui_ctx.clone(),
                std::env::var_os("DEVPULSE_DESKTOP_CONFIG")
                    .map(Into::into)
                    .or_else(settings::default_path),
                make_store,
                Overrides::from_env(),
                sys_locale::get_locale(),
                missing_cjk_font,
            )))
        }),
    )
}
