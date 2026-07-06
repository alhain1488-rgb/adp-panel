// Single source of truth for the displayed app version.
// Scheme: MAJOR.MINOR.PATCH.BUILD — bump BUILD for small tweaks/fixes, PATCH for
// user-facing features, MINOR at phase boundaries. Claude bumps this on EVERY
// change (see CLAUDE.md "Версионность").
export const APP_VERSION = '0.9.5.0'
