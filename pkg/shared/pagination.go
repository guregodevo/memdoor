package shared

const (
	DefaultMessagePageSize = 50
	MaxMessagePageSize     = 500
	DefaultListPageSize    = 50
	MaxListPageSize        = 200
)

// CursorPage is cursor-based pagination for ordered feeds (messages).
// BeforeID = 0 means "get the most recent page."
type CursorPage struct {
	BeforeID int64
	Limit    int
}

// OffsetPage is offset-based pagination for entity lists (channels, members, agents).
type OffsetPage struct {
	Offset int
	Limit  int
}

func NewCursorPage(beforeID int64, limit int) CursorPage {
	if limit <= 0 {
		limit = DefaultMessagePageSize
	}
	if limit > MaxMessagePageSize {
		limit = MaxMessagePageSize
	}
	return CursorPage{BeforeID: beforeID, Limit: limit}
}

// AllPages returns an OffsetPage that fetches up to MaxListPageSize items.
func AllPages() OffsetPage {
	return OffsetPage{Offset: 0, Limit: MaxListPageSize}
}
