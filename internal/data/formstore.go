package data

import "sync"

// The choices last made in each keybinding's form, kept in
// $XDG_STATE_HOME/gh-dash/forms.json so the next one starts from them.

var (
	formsMu sync.Mutex
	forms   map[string]map[string]string
)

func loadForms() {
	if forms == nil {
		forms = map[string]map[string]string{}
		LoadState("forms.json", &forms)
	}
}

// GetFormChoices is the field name -> option last chosen in form, if any.
func GetFormChoices(form string) map[string]string {
	formsMu.Lock()
	defer formsMu.Unlock()
	loadForms()
	return forms[form]
}

// SetFormChoices saves the choices made in form.
func SetFormChoices(form string, values map[string]string) {
	formsMu.Lock()
	defer formsMu.Unlock()
	loadForms()
	forms[form] = values
	SaveState("forms.json", forms)
}
