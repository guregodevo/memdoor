package filestore

// Config holds file sharing settings
type Config struct {
	MaxFileSizeMB int // Per-file upload limit (default: 25)
	MaxStorageMB  int // Total storage cap (default: 1024)
	RetentionDays int // Auto-delete after N days, 0 = keep forever (default: 90)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxFileSizeMB: 25,
		MaxStorageMB:  1024,
		RetentionDays: 90,
	}
}
