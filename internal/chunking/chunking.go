package chunking

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/dmvianna/chatbot-prototype/internal/types"
)

// SplitText chunks text with overlap for retrieval.
func SplitText(path, text string, chunkSize, overlap int) []types.Chunk {
	if chunkSize <= 0 {
		chunkSize = 800
	}
	if overlap < 0 || overlap >= chunkSize {
		overlap = chunkSize / 5
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	var out []types.Chunk
	step := chunkSize - overlap
	for i := 0; i < len(runes); i += step {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunkText := strings.TrimSpace(string(runes[i:end]))
		if chunkText == "" {
			continue
		}
		id := makeID(path, i, chunkText)
		out = append(out, types.Chunk{ID: id, Citation: id, Path: path, Text: chunkText})
		if end == len(runes) {
			break
		}
	}
	return out
}

func makeID(path string, offset int, text string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%s|%d", path, text, offset)))
	return "chunk_" + hex.EncodeToString(h[:8])
}
