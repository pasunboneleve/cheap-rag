package chunking

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dmvianna/chatbot-prototype/internal/types"
)

// FormatCitation builds a citation token from a template.
// Supported placeholders: {chunk_id}, {path}, {slug}, {chunk_index}.
func FormatCitation(pattern string, chunk types.Chunk, chunkIndex int) string {
	if strings.TrimSpace(pattern) == "" {
		pattern = "{chunk_id}"
	}
	slug := strings.TrimSuffix(path.Base(filepath.ToSlash(chunk.Path)), filepath.Ext(chunk.Path))
	repl := strings.NewReplacer(
		"{chunk_id}", chunk.ID,
		"{path}", filepath.ToSlash(chunk.Path),
		"{slug}", slug,
		"{chunk_index}", strconv.Itoa(chunkIndex),
	)
	out := strings.TrimSpace(repl.Replace(pattern))
	if out == "" {
		return fmt.Sprintf("%s#%d", chunk.ID, chunkIndex)
	}
	return out
}
