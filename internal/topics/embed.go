package topics

import _ "embed"

// defaultTopicsJSON is the built-in catalog. Override at runtime with TOPICS_PATH.
//
//go:embed topics.json
var defaultTopicsJSON []byte
