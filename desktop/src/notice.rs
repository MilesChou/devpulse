//! Messages shown under the top bar. Kept as data rather than text so a
//! message on screen follows a language switch, and so `state` can
//! decide what to say without knowing the UI language.

use crate::api::ApiError;
use crate::i18n::Texts;

/// Which repo label a validation message is about.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Label {
    Incident,
    Hotfix,
}

#[derive(Debug, Clone, PartialEq)]
pub enum Notice {
    // Connection and period.
    KeychainReadFailed(String),
    UrlRequired,
    TokenSaved,
    TokenSaveFailed(String),
    TokenRemoved,
    TokenRemoveFailed(String),
    SettingsSaveFailed(String),
    FromBeforeTo,
    BadMonth(String),

    // Repos.
    RepoAdded(String),
    RepoAddedWithoutMetadata { repo: String, err: String },
    RepoAlreadyTracked(String),
    RepoAddFailed(ApiError),
    RepoSettingsSaved(String),
    RepoSettingsFailed(ApiError),
    RepoRemoved(String),
    RepoRemoveFailed(String, ApiError),
    PrStartInvalid(String),
    LabelBlank(Label),

    // Sync.
    Synced(String),
    SyncFailed { repo: String, err: String },
    SyncRequestFailed(ApiError),

    // People.
    MemberSaved(String),
    TeamSaved(String),
    MemberDeleted(String),
    TeamDeleted(String),
    PeopleChangeFailed(ApiError),
    DisplayNameBlank,
    TeamNameBlank,
    ExcludedSaved,
    ExcludedSaveFailed(ApiError),
}

impl Notice {
    pub fn is_error(&self) -> bool {
        !matches!(
            self,
            Self::TokenSaved
                | Self::TokenRemoved
                | Self::RepoAdded(_)
                | Self::RepoAlreadyTracked(_)
                | Self::RepoSettingsSaved(_)
                | Self::RepoRemoved(_)
                | Self::Synced(_)
                | Self::MemberSaved(_)
                | Self::TeamSaved(_)
                | Self::MemberDeleted(_)
                | Self::TeamDeleted(_)
                | Self::ExcludedSaved
        )
    }

    pub fn text(&self, t: &Texts) -> String {
        match self {
            Self::KeychainReadFailed(e) => (t.keychain_read_failed)(e),
            Self::UrlRequired => t.url_required.into(),
            Self::TokenSaved => t.token_saved.into(),
            Self::TokenSaveFailed(e) => (t.token_save_failed)(e),
            Self::TokenRemoved => t.token_removed.into(),
            Self::TokenRemoveFailed(e) => (t.token_remove_failed)(e),
            Self::SettingsSaveFailed(e) => (t.settings_save_failed)(e),
            Self::FromBeforeTo => t.from_before_to.into(),
            Self::BadMonth(input) => (t.bad_month)(input),

            Self::RepoAdded(repo) => (t.repo_added)(repo),
            Self::RepoAddedWithoutMetadata { repo, err } => (t.repo_added_no_metadata)(repo, err),
            Self::RepoAlreadyTracked(repo) => (t.repo_already_tracked)(repo),
            Self::RepoAddFailed(e) => (t.repo_add_failed)(&e.describe(t)),
            Self::RepoSettingsSaved(repo) => (t.repo_settings_saved)(repo),
            Self::RepoSettingsFailed(e) => (t.repo_settings_failed)(&e.describe(t)),
            Self::RepoRemoved(repo) => (t.repo_removed)(repo),
            Self::RepoRemoveFailed(repo, e) => (t.repo_remove_failed)(repo, &e.describe(t)),
            Self::PrStartInvalid(input) => (t.pr_start_invalid)(input),
            Self::LabelBlank(Label::Incident) => (t.label_blank)(t.col_incident_label),
            Self::LabelBlank(Label::Hotfix) => (t.label_blank)(t.col_hotfix_label),

            Self::Synced(repo) => (t.synced)(repo),
            Self::SyncFailed { repo, err } => (t.sync_failed)(repo, err),
            Self::SyncRequestFailed(e) => (t.sync_request_failed)(&e.describe(t)),

            Self::MemberSaved(name) => (t.member_saved)(name),
            Self::TeamSaved(name) => (t.team_saved)(name),
            Self::MemberDeleted(name) => (t.member_deleted)(name),
            Self::TeamDeleted(name) => (t.team_deleted)(name),
            Self::PeopleChangeFailed(e) => e.describe(t),
            Self::DisplayNameBlank => t.display_name_blank.into(),
            Self::TeamNameBlank => t.team_name_blank.into(),
            Self::ExcludedSaved => t.excluded_saved.into(),
            Self::ExcludedSaveFailed(e) => (t.excluded_save_failed)(&e.describe(t)),
        }
    }
}
