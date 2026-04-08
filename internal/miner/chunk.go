package miner

import "strings"

// ChunkText splits text into chunks of at most maxSize characters,
// preferring paragraph boundaries (double newlines). Each new chunk
// begins with overlap characters from the end of the previous chunk
// to preserve context across boundaries.
func ChunkText(text string, maxSize, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= maxSize {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// If adding this paragraph exceeds maxSize, finalize current chunk
		if current.Len() > 0 && current.Len()+len(para)+2 > maxSize {
			chunk := strings.TrimSpace(current.String())
			chunks = append(chunks, chunk)

			// Start new chunk with overlap from end of previous
			current.Reset()
			if overlap > 0 && len(chunk) > overlap {
				current.WriteString(chunk[len(chunk)-overlap:])
				current.WriteString("\n\n")
			}
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}
	return chunks
}
