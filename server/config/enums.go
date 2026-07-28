package config

// Closed manifest value sets. These are the single source of truth for three
// consumers that used to carry their own copies: runtime validation through
// requireOneOf, the `jsonschema:"enum=…"` struct tags that produce the editor
// schema, and the generated example manifests. enum_contract_test.go asserts
// the struct tags still match these lists, so adding a value in one place and
// forgetting the other fails the build rather than shipping a schema that
// rejects a config the server accepts.
var (
	environmentValues   = []string{"development", "production", "test"}
	tlsModeValues       = []string{string(TLSModeOff), string(TLSModeACME), string(TLSModeExternal)}
	proxyModeValues     = []string{string(ProxyModeDisabled), string(ProxyModeRequired)}
	logLevelValues      = []string{"debug", "info", "warn", "error"}
	logFormatValues     = []string{"console", "json"}
	geocodingProviders  = []string{"disabled", "nominatim"}
	hardwareAccelValues = []string{"auto", "vaapi", "nvenc", "qsv", "videotoolbox", "none"}
)
