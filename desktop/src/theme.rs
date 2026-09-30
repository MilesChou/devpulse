//! The dashboard's look: one accent colour, rounded corners, sections
//! on cards, and the status / change colours every page shares. Each
//! colour is picked for the light and the dark theme, so following the
//! OS theme keeps everything readable.

use eframe::egui::{
    self, Color32, CornerRadius, FontFamily, FontId, Frame, InnerResponse, Margin, Shadow, Stroke,
    TextStyle, Theme, Ui,
};

/// Corner radius of cards.
const CARD_RADIUS: u8 = 10;
/// Corner radius of buttons, inputs and selectable labels.
const WIDGET_RADIUS: u8 = 6;

fn accent(dark: bool) -> Color32 {
    if dark {
        Color32::from_rgb(0x8E, 0xA2, 0xFF)
    } else {
        Color32::from_rgb(0x4F, 0x6B, 0xED)
    }
}

/// Applies text sizes and visuals to both themes.
pub fn apply(ctx: &egui::Context) {
    for theme in [Theme::Light, Theme::Dark] {
        ctx.style_mut_of(theme, |style| {
            let dark = theme == Theme::Dark;
            // egui's defaults (small 9, body 13, heading 18) are too small
            // for card details and chart captions. Cmd/Ctrl + and - still
            // zoom the whole UI on top of this.
            style.text_styles = [
                (
                    TextStyle::Small,
                    FontId::new(12.0, FontFamily::Proportional),
                ),
                (TextStyle::Body, FontId::new(15.0, FontFamily::Proportional)),
                (
                    TextStyle::Button,
                    FontId::new(15.0, FontFamily::Proportional),
                ),
                (
                    TextStyle::Heading,
                    FontId::new(21.0, FontFamily::Proportional),
                ),
                (
                    TextStyle::Monospace,
                    FontId::new(14.0, FontFamily::Monospace),
                ),
            ]
            .into();

            style.spacing.item_spacing = egui::vec2(8.0, 6.0);
            style.spacing.button_padding = egui::vec2(10.0, 5.0);

            let v = &mut style.visuals;
            let a = accent(dark);
            v.hyperlink_color = a;
            v.selection.bg_fill = a.gamma_multiply(if dark { 0.45 } else { 0.25 });
            v.selection.stroke = Stroke::new(1.0, a);
            // Cards sit on the panel, so the panel is a shade off them.
            if dark {
                v.panel_fill = Color32::from_rgb(0x16, 0x18, 0x1D);
                v.extreme_bg_color = Color32::from_rgb(0x22, 0x25, 0x2C);
                v.faint_bg_color = Color32::from_rgb(0x1D, 0x20, 0x26);
            } else {
                v.panel_fill = Color32::from_rgb(0xF3, 0xF4, 0xF7);
                v.extreme_bg_color = Color32::WHITE;
                v.faint_bg_color = Color32::from_rgb(0xF7, 0xF8, 0xFB);
            }
            v.window_fill = v.extreme_bg_color;
            v.window_corner_radius = CornerRadius::same(CARD_RADIUS);
            v.menu_corner_radius = CornerRadius::same(WIDGET_RADIUS);
            for w in [
                &mut v.widgets.noninteractive,
                &mut v.widgets.inactive,
                &mut v.widgets.hovered,
                &mut v.widgets.active,
                &mut v.widgets.open,
            ] {
                w.corner_radius = CornerRadius::same(WIDGET_RADIUS);
            }
            v.widgets.hovered.bg_stroke = Stroke::new(1.0, a.gamma_multiply(0.6));
            v.widgets.active.bg_stroke = Stroke::new(1.0, a);
        });
    }
}

/// A section on a card: its own background, hairline border and a soft
/// shadow, so pages read as groups instead of one flat sheet.
pub fn card<R>(ui: &mut Ui, add_contents: impl FnOnce(&mut Ui) -> R) -> InnerResponse<R> {
    let dark = ui.visuals().dark_mode;
    Frame::new()
        .fill(ui.visuals().extreme_bg_color)
        .stroke(Stroke::new(
            1.0,
            if dark {
                Color32::from_white_alpha(18)
            } else {
                Color32::from_black_alpha(18)
            },
        ))
        .corner_radius(CornerRadius::same(CARD_RADIUS))
        .inner_margin(Margin::same(14))
        .shadow(Shadow {
            offset: [0, 1],
            blur: 6,
            spread: 0,
            color: Color32::from_black_alpha(if dark { 60 } else { 14 }),
        })
        .show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            add_contents(ui)
        })
}

/// A value's standing, for status bars and change tags.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Tone {
    Good,
    Near,
    Bad,
    Neutral,
}

impl Tone {
    /// Foreground colour: text and bars.
    pub fn color(self, ui: &Ui) -> Color32 {
        let dark = ui.visuals().dark_mode;
        match (self, dark) {
            (Tone::Good, false) => Color32::from_rgb(0x1F, 0x8A, 0x4C),
            (Tone::Good, true) => Color32::from_rgb(0x5F, 0xD3, 0x8D),
            (Tone::Near, false) => Color32::from_rgb(0xB7, 0x79, 0x0F),
            (Tone::Near, true) => Color32::from_rgb(0xF2, 0xC0, 0x5C),
            (Tone::Bad, false) => Color32::from_rgb(0xC9, 0x3C, 0x2F),
            (Tone::Bad, true) => Color32::from_rgb(0xFF, 0x86, 0x78),
            (Tone::Neutral, _) => ui.visuals().weak_text_color(),
        }
    }

    /// A light tint of the colour, for tag backgrounds.
    pub fn tint(self, ui: &Ui) -> Color32 {
        match self {
            Tone::Neutral => ui.visuals().faint_bg_color,
            _ => self
                .color(ui)
                .gamma_multiply(if ui.visuals().dark_mode { 0.22 } else { 0.14 }),
        }
    }
}

/// Series colours for charts: the accent first, then colours that stay
/// apart from it and from each other in both themes.
pub fn series(ui: &Ui, i: usize) -> Color32 {
    let dark = ui.visuals().dark_mode;
    let palette = if dark {
        [
            accent(true),
            Color32::from_rgb(0xF2, 0xA6, 0x5A),
            Color32::from_rgb(0x5F, 0xD3, 0xB4),
        ]
    } else {
        [
            accent(false),
            Color32::from_rgb(0xE0, 0x7B, 0x28),
            Color32::from_rgb(0x1C, 0x9C, 0x80),
        ]
    };
    palette[i % palette.len()]
}
