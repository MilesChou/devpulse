//! Finds a system font that can draw Traditional Chinese.
//!
//! egui's bundled fonts have no CJK glyphs. The font found here is
//! appended to the end of egui's font families, so Latin text keeps the
//! default look and only glyphs the defaults lack (Chinese UI strings,
//! Chinese repo names and descriptions from the API) come from it.

use std::sync::Arc;

use eframe::egui::{self, FontData, FontDefinitions, FontFamily};

/// Candidate family names, in order of preference, per platform.
#[cfg(target_os = "macos")]
const CANDIDATES: &[&str] = &["PingFang TC", "Heiti TC", "Hiragino Sans CNS"];
#[cfg(target_os = "windows")]
const CANDIDATES: &[&str] = &["Microsoft JhengHei", "Microsoft JhengHei UI"];
#[cfg(not(any(target_os = "macos", target_os = "windows")))]
const CANDIDATES: &[&str] = &[
    "Noto Sans CJK TC",
    "Noto Sans TC",
    "Source Han Sans TC",
    "WenQuanYi Zen Hei",
];

const FONT_NAME: &str = "cjk";

/// The candidate list, for telling the user what was searched.
pub fn candidates() -> String {
    CANDIDATES.join(", ")
}

/// A loaded CJK face.
pub struct CjkFont {
    data: Vec<u8>,
    index: u32,
}

/// Searches the system fonts for the first available candidate.
pub fn find_cjk() -> Option<CjkFont> {
    let mut db = fontdb::Database::new();
    db.load_system_fonts();
    CANDIDATES.iter().find_map(|&name| {
        let id = db.query(&fontdb::Query {
            families: &[fontdb::Family::Name(name)],
            ..Default::default()
        })?;
        db.with_face_data(id, |data, index| CjkFont {
            data: data.to_vec(),
            index,
        })
    })
}

/// Appends `font` as the last fallback of the proportional and
/// monospace families.
pub fn install(ctx: &egui::Context, font: CjkFont) {
    let mut defs = FontDefinitions::default();
    let mut data = FontData::from_owned(font.data);
    data.index = font.index;
    defs.font_data.insert(FONT_NAME.into(), Arc::new(data));
    for family in [FontFamily::Proportional, FontFamily::Monospace] {
        defs.families
            .entry(family)
            .or_default()
            .push(FONT_NAME.into());
    }
    ctx.set_fonts(defs);
}
