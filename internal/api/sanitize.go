package api

import (
	"regexp"
	"strings"
	"fmt"
)

var (
	// Firecast color codes: $FFFFFFFF, $FFB9BABC, etc.
	reColorCode = regexp.MustCompile(`\$[0-9A-Fa-f]{6,8}`)
	// Font declarations: "Roboto txt", "Arial txt", etc.
	reFontDecl = regexp.MustCompile(`(?i)\b(Roboto|Arial|Verdana|Segoe UI|Times New Roman|Calibri|Tahoma|Helvetica)\s+txt\b`)
	// Encoding/version headers: "1.0 UTF-8", "1.0 utf-8"
	reEncoding = regexp.MustCompile(`(?m)^\s*1\.0\s+UTF-8\s*`)
	// Timestamp lines from Firecast chat: "03/07/2021 - 15:06 — narrado por Angelloh"
	reTimestamp = regexp.MustCompile(`\d{2}/\d{2}/\d{4}\s*-\s*\d{2}:\d{2}\s*—\s*\S+\s+por\s+\S+`)
	// Bullet markers
	reBullet = regexp.MustCompile(`(?m)^\s*●\s*`)
	// Recurring boilerplate: session schedule block ("Grupo 1 - Sessões às Quintas...")
	reScheduleBlock = regexp.MustCompile(`(?i)Grupo\s+\d+\s*-\s*Sess[oõ]es\s+às\s+\w+[^\n]*`)
	// Recurring boilerplate: waitlist paragraph
	reWaitlistBlock = regexp.MustCompile(`(?i)(?:Lista de Espera|candidat[ao]r[- ]se|disponibilidade completa|pedir uma ficha para o Mestre)[^\n]*`)
	// Discord invite links
	reDiscordLink = regexp.MustCompile(`https?://discord\.gg/\S+`)
	// Collapse multiple whitespace
	reMultiSpace = regexp.MustCompile(`[ \t]{2,}`)
	// Collapse 3+ newlines into 2
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
	// Metadata prefix lines for stripping before LLM context
	reMetaPrefix = regexp.MustCompile(`(?m)^\[(?:Path|Type):\s*[^\]]*\]\s*\n?`)
	reTitlePrefix = regexp.MustCompile(`(?m)^Title:\s*[^\n]*\n?`)
)

func sanitizeFirecastText(text string) string {
	text = reColorCode.ReplaceAllString(text, "")
	text = reFontDecl.ReplaceAllString(text, "")
	text = reEncoding.ReplaceAllString(text, "")
	text = reTimestamp.ReplaceAllString(text, "")
	text = reBullet.ReplaceAllString(text, "")
	text = reScheduleBlock.ReplaceAllString(text, "")
	text = reWaitlistBlock.ReplaceAllString(text, "")
	text = reDiscordLink.ReplaceAllString(text, "")
	text = reMultiSpace.ReplaceAllString(text, " ")
	text = reMultiNewline.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)
	return text
}

var reGroupMention = regexp.MustCompile(`(?i)\bgrupo\s+(\d+)\b`)
var reArcMention = regexp.MustCompile(`(?i)\barco\s+(\d+)\b`)

// extractPathFilter scans the query for explicit group or arc references and
// returns a path substring to use as a WHERE ILIKE filter, narrowing retrieval
// to the relevant section of the library. Returns "" if no signal found.
func extractPathFilter(query string) string {
	if m := reGroupMention.FindStringSubmatch(query); m != nil {
		return fmt.Sprintf("Grupo %s", m[1])
	}
	if m := reArcMention.FindStringSubmatch(query); m != nil {
		return fmt.Sprintf("Arco %s", m[1])
	}
	return ""
}

func stripMetadataForLLM(content string) string {
	text := reMetaPrefix.ReplaceAllString(content, "")
	text = reTitlePrefix.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	return text
}
