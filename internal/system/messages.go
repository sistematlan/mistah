package system

import (
	"os"
	"path/filepath"
	"time"

	"github.com/sistematlan/mistah/internal/item"
)

// messagesAttachmentMaxAgeDays is the cutoff for iMessage attachments.
// Only attachments older than this are candidates. Recent conversations
// stay fully intact. 180 days (6 months) is deliberately conservative:
// far enough back that the user almost certainly won't scroll there, but
// short enough to still reclaim meaningful space on a chatty Mac.
//
// Mutable so tests can lower it without back-dating files by 6 months.
var messagesAttachmentMaxAgeDays = 180

// scanMessagesAttachments reports old iMessage attachments as a single
// aggregated item.
//
// ~/Library/Messages/Attachments/ stores every photo, video, audio clip
// and file ever sent or received over iMessage, in a hashed directory
// tree (ab/cd/<guid>/file). On a Mac with heavy iMessage use this can be
// 10-40 GB.
//
// Critical design choices:
//
//   - We NEVER touch chat.db (the SQLite store of the conversations
//     themselves). Only the Attachments tree. The text of every chat
//     survives untouched; only the media previews for OLD messages go.
//   - RiskAskBefore, ALWAYS. A backup may be the only copy of a photo
//     someone sent years ago. The wizard routes this to per-item review;
//     it never auto-deletes.
//   - One aggregated item, not one per file. Attachments number in the
//     thousands; listing each would drown the UI. We report the total
//     bytes and count of attachments older than the cutoff, and the
//     cleaner's OldFilesRemover (recursive, match-all, age-filtered)
//     does the actual deletion with the same cutoff.
//
// Returns nil (no item) when the directory is missing, unreadable
// (Messages lives behind macOS TCC — without Full Disk Access the walk
// fails, which we treat as "nothing to offer"), or holds no old files.
func scanMessagesAttachments(home string) []item.Item {
	dir := filepath.Join(home, "Library", "Messages", "Attachments")
	bytes, count, ok := summarizeOldAttachments(dir, messagesAttachmentMaxAgeDays)
	if !ok || bytes <= 0 || count == 0 {
		return nil
	}
	months := messagesAttachmentMaxAgeDays / 30
	return []item.Item{{
		Name:       "iMessage attachments",
		Tool:       "ios-messages",
		Path:       dir,
		Bytes:      bytes,
		Category:   item.CategorySystem,
		Risk:       item.RiskAskBefore,
		Detail:     "adjuntos antiguos de iMessage; los chats no se borran, solo el archivo adjunto",
		DetailKey:  "system.imessage.detail",
		DetailArgs: []any{count, months},
	}}
}

// summarizeOldAttachments walks the Attachments tree and totals the size
// and count of files older than maxDays. It does NOT delete anything —
// it only measures, so the wizard can show an accurate "X.X GB across N
// attachments" before the user decides.
//
// Walks the whole subtree (attachments are nested under hashed dirs).
// Per-node errors are swallowed: an unreadable subdir contributes
// nothing rather than aborting the measurement. Returns ok=false only
// when the root itself can't be opened (missing dir or TCC denial).
func summarizeOldAttachments(root string, maxDays int) (bytes int64, count int, ok bool) {
	// Probe the root first so a missing/denied directory returns ok=false
	// distinctly from an empty-but-present one.
	if _, err := os.Stat(root); err != nil {
		return 0, 0, false
	}

	cutoff := time.Now().Add(-time.Duration(maxDays) * 24 * time.Hour)
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable node, keep walking
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(cutoff) {
			return nil // newer than cutoff: keep
		}
		bytes += info.Size()
		count++
		return nil
	})
	if walkErr != nil {
		// A hard walk failure at the root is the TCC/permission case.
		return 0, 0, false
	}
	return bytes, count, true
}
