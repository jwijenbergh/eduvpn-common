package err

// Translated defines the type for translated strings
// It is a map from language tags to error messages
type Translated map[string]string

// Error is the struct that defines the public error types
// This contains the error message with translations
// And other info
type Error struct {
	// Message defines the error message
	// It is a map from language tags to messages
	// If a language is not translated, the whole language tag key is missing
	// E.g. compare (english and french translations)
	// {"en": "hello", "fr": "bonjour"}
	// and
	// {"en": "hello"}
	// English is always present and should be used as a fallback
	Message Translated `json:"message"`

	// Misc indicates whether or not this error is only there for miscellaneous purposes
	// If this is set to True, the client UI SHOULD NOT show this error
	Misc bool `json:"misc"`
}
