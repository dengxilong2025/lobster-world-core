package gateway

import _ "embed"

// uiPageHTML is a minimal single-page web shell for v0.2-M1.
//
// Notes:
// - This page is served by the Go backend (no extra frontend build tool).
// - Page text is allowed; the "no text" rule applies to art/image assets only.
// - Keep DOM ids stable so agentic testers can script interactions.
//
//go:embed ui_page.html
var uiPageHTML string
