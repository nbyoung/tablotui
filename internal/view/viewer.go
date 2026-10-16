package view

// Viewer is the person at the keyboard as tablo's envelope states them: the
// email, "" for nobody where the envelope writes null, and the roles that
// email holds anywhere in the project, in tablo's order. Roles is
// "observer" alone when the email holds none.
type Viewer struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}
