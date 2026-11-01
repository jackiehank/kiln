package cli

// personDoc is how a commit's git author appears in `--json`. It is one object
// rather than two flat keys so a later `role` / `signature` can join it without
// reshaping the field; jsonl pays one level of nesting for that.
//
// kiln never writes an author trailer: the identity is the commit's own, and the
// read side only projects it back out (see docs/json-author.md).
type personDoc struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// personOf builds the author object, or nil when the backend reported nothing —
// "缺省即省略" keeps an unknown author out of the document instead of leaving an
// empty object behind.
func personOf(name, email string) *personDoc {
	if name == "" && email == "" {
		return nil
	}
	return &personDoc{Name: name, Email: email}
}
