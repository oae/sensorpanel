package music

import _ "embed"

// DashboardHTML is the self-contained now-playing dashboard.
//
//go:embed dashboard/index.html
var DashboardHTML []byte
