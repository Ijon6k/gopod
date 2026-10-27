package storage

import "fmt"

// formatSnapshotSize renders an on-disk backup size for UI display. Volume
// snapshots are frequently just a few kilobytes of tar and gzip headers, so
// sub-MB files keep a KB unit instead of rounding away to "0.00 MB".
func formatSnapshotSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.0f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
