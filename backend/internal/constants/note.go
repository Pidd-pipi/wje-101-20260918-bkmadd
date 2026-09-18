package constants

// RoastLevel enumerates coffee roast levels.
const (
	RoastLight  = "light"
	RoastMedium = "medium"
	RoastDark   = "dark"
)

// NoteStatus enumerates tasting note publication states.
const (
	NoteStatusDraft     = "draft"
	NoteStatusPublished = "published"
)

// ValidRoastLevels returns all accepted roast levels.
func ValidRoastLevels() []string {
	return []string{RoastLight, RoastMedium, RoastDark}
}

// IsValidRoastLevel reports whether a level is known.
func IsValidRoastLevel(s string) bool {
	for _, v := range ValidRoastLevels() {
		if v == s {
			return true
		}
	}
	return false
}

// IsValidNoteStatus reports whether a note status is known.
func IsValidNoteStatus(s string) bool {
	return s == NoteStatusDraft || s == NoteStatusPublished
}
