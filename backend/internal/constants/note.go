package constants

// RoastLevel enumerates coffee roast levels.
const (
	RoastLight  = "light"
	RoastMedium = "medium"
	RoastDark   = "dark"
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

// Note publication status.
const (
	NoteStatusDraft     = "draft"
	NoteStatusPublished = "published"
)

// ValidNoteStatuses returns all accepted note statuses.
func ValidNoteStatuses() []string {
	return []string{NoteStatusDraft, NoteStatusPublished}
}

// IsValidNoteStatus reports whether a status is known.
func IsValidNoteStatus(s string) bool {
	for _, v := range ValidNoteStatuses() {
		if v == s {
			return true
		}
	}
	return false
}
