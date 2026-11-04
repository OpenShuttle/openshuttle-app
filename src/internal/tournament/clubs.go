package tournament

import (
	"encoding/json"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Club is an entry of the club list of a PC (app.db): its name and an
// optional acronym ("Rillieux", "AUVR"). A player's club matches it by
// either, case, accents, spaces and punctuation ignored.
type Club struct {
	Name    string `json:"name"`
	Acronym string `json:"acronym"`
}

// UnmarshalJSON also reads the plain club names of older club lists.
func (c *Club) UnmarshalJSON(data []byte) error {
	var name string
	if json.Unmarshal(data, &name) == nil {
		*c = Club{Name: name}
		return nil
	}
	type plain Club
	return json.Unmarshal(data, (*plain)(c))
}

// ClubDisplay says how clubs are shown: by name, or by acronym when the
// club has one.
type ClubDisplay string

const (
	ClubDisplayName    ClubDisplay = "name"
	ClubDisplayAcronym ClubDisplay = "acronym"
)

// ClubKey is what club values are compared on: letters and digits only,
// lowercased, accents dropped. "Rillieux-la-Pape" and "rillieux la pape"
// share a key, as "A.U.V.R." and "AUVR".
func ClubKey(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ClubBook resolves the clubs typed for players against a club list, and
// shows them as the settings say. The zero ClubBook knows no club and shows
// clubs as typed.
type ClubBook struct {
	list    []Club
	index   map[string]int // key of a name or acronym → list index
	display ClubDisplay
}

// NewClubBook indexes a club list. A name comes before an acronym: when a
// value is one club's name and another's acronym, it is the first club.
func NewClubBook(list []Club, display ClubDisplay) ClubBook {
	b := ClubBook{list: list, index: map[string]int{}, display: display}
	for _, pass := range []func(Club) string{
		func(c Club) string { return c.Name },
		func(c Club) string { return c.Acronym },
	} {
		for i, c := range list {
			if k := ClubKey(pass(c)); k != "" {
				if _, taken := b.index[k]; !taken {
					b.index[k] = i
				}
			}
		}
	}
	return b
}

// Find returns the club of the list a value names, by name or acronym.
func (b ClubBook) Find(value string) (Club, bool) {
	i, ok := b.index[ClubKey(value)]
	if !ok {
		return Club{}, false
	}
	return b.list[i], true
}

// Name is the name of the listed club a value names, else the value.
func (b ClubBook) Name(value string) string {
	if c, ok := b.Find(value); ok {
		return c.Name
	}
	return value
}

// ID identifies the club of a value: two values naming the same listed
// club, or spelled alike, share it. "" for no club.
func (b ClubBook) ID(value string) string {
	return ClubKey(b.Name(value))
}

// Label is how a club is shown: its acronym when the settings say so and
// the club has one, otherwise its name.
func (b ClubBook) Label(value string) string {
	c, ok := b.Find(value)
	if !ok {
		return value
	}
	if b.display == ClubDisplayAcronym && c.Acronym != "" {
		return c.Acronym
	}
	return c.Name
}
