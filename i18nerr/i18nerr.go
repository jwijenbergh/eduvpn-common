// package i18nerr implements errors with internationalization using gotext
package i18nerr

import (
	"context"
	"errors"
	"sync"

	"github.com/eduvpn/eduvpn-common/internal/log"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var printers sync.Map
var once sync.Once


// TranslatedInner defines errors that are used as inner causes but are still translated because they can happen frequently
func TranslatedInner(inner error) (string, bool) {
	unwrapped := inner
	for errors.Unwrap(unwrapped) != nil {
		unwrapped = errors.Unwrap(unwrapped)
	}

	switch {
	case errors.Is(inner, context.DeadlineExceeded):
		return printerOrNew(language.English).Sprintf("timeout reached"), false
	case errors.Is(inner, context.Canceled):
		return unwrapped.Error(), true
	}
	return unwrapped.Error(), false
}

// Error wraps an actual error with the translation key
// This translation key is later used to lookup translation
// The inner error always consists of the translation key and some formatting
type Error struct {
	key message.Reference
	args []interface{}
	wrapped *Error
	Misc bool
}

func (e *Error) translated(t language.Tag) string {
	once.Do(func() {
		inititializeLangs()
	})
	msg := printerOrNew(t).Sprintf(e.key, e.args)
	if e.wrapped != nil {
		return msg + " " + printerOrNew(t).Sprintf("with cause:") + " " + e.wrapped.Error()
	}
	return msg
}

// Error gets the error string
// it does this by simply forwarding the error method from the actual inner error
func (e *Error) Error() string {
	return e.translated(language.English)
}

// Translations returns all the translations for the error including the source translation (english)
func (e *Error) Translations() map[string]string {
	translations := make(map[string]string)
	// add the source transltaion first
	source := e.Error()
	translations[language.English.String()] = source
	for _, t := range message.DefaultCatalog.Languages() {
		// already added
		if t == language.English {
			continue
		}
		// get the final translation string for the tag
		// and add it if it's not equal to the english version
		f := e.translated(t)
		if f != source {
			translations[t.String()] = f
		}
	}
	return translations
}


// Unwrap returns the unwrapped error
// it does this by unwrapping the inner error
func (e *Error) Unwrap() error {
	if e.wrapped == nil {
		return nil
	}
	return e.wrapped.Unwrap()
}

// printerOrNew gets a message printer from the global printers map using the tag 'tag'
// If the printer cannot be found in the sync map, we return a new printer
func printerOrNew(tag language.Tag) *message.Printer {
	v, ok := printers.Load(tag)
	if !ok {
		log.Logger.Debugf("i18n: could not load printer with tag: '%v' from map", tag)
		return message.NewPrinter(tag)
	}
	p, ok := v.(*message.Printer)
	if !ok {
		log.Logger.Debugf("i18n: could not load printer with tag: '%v' as the type is not correct: '%T'", tag, p)
		return message.NewPrinter(tag)
	}
	return p
}

// New creates a new i18n error using a message reference
func New(key message.Reference) *Error {
	_ = printerOrNew(language.English).Sprint(key)
	return &Error{key: key}
}

// Newf creates a new i18n error using a message reference and arguments.
// It formats this with fmt.Errorf
func Newf(key message.Reference, args ...interface{}) *Error {
	_ = printerOrNew(language.English).Sprintf(key, args...)
	return &Error{key: key, args: args}
}

// Wrap creates a new i18n error using an error to be wrapped 'err' and a prefix message reference 'key'.
// It formats this with fmt.Errorf
func Wrap(err error, key message.Reference) *Error {
	_ = printerOrNew(language.English).Sprintf(key)
	t, misc := TranslatedInner(err)
	return &Error{key: key, wrapped: &Error{key: t, Misc: misc}, Misc: misc}
}

// Wrapf creates a new i18n error using an error to be wrapped 'err' and a prefix message reference 'key' with format arguments 'args'.
// It formats this with fmt.Errorf
func Wrapf(err error, key message.Reference, args ...interface{}) *Error {
	_ = printerOrNew(language.English).Sprintf(key, args...)
	t, misc := TranslatedInner(err)
	return &Error{key: key, args: args, wrapped: &Error{key: t, Misc: misc}, Misc: misc}
}

// initializeLangs initializes the printers from the default catalog into the sync map
// we cannot do this in init() because this is too early
func inititializeLangs() {
	log.Logger.Debugf("i18n: initializing languages...")
	for _, t := range message.DefaultCatalog.Languages() {
		printers.Store(t, message.NewPrinter(t))
	}
}
