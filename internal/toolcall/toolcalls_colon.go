package toolcall

import "strings"

var colonToolNames = []struct{ raw, canonical string }{
	{"tool_calls", "tool_calls"}, {"tool-calls", "tool_calls"}, {"toolcalls", "tool_calls"},
	{"parameter", "parameter"}, {"invoke", "invoke"}, {"param", "parameter"}, {"tc", "tool_calls"},
}

func scanColonToolMarkupTagAt(text string, start int) (ToolMarkupTag, bool) {
	if start < 0 || !strings.HasPrefix(text[start:], "::") {
		return ToolMarkupTag{}, false
	}
	i := start + 2
	closing := false
	if i < len(text) && text[i] == '/' {
		closing = true
		i++
	}
	nameStart := i
	name, nameEnd := "", i
	for _, candidate := range colonToolNames {
		end := i + len(candidate.raw)
		if end <= len(text) && strings.EqualFold(text[i:end], candidate.raw) && (end == len(text) || strings.ContainsRune(" \t\r\n:", rune(text[end]))) {
			name, nameEnd = candidate.canonical, end
			break
		}
	}
	if name == "" {
		return ToolMarkupTag{}, false
	}
	quote := byte(0)
	for pos := nameEnd; pos+1 < len(text); pos++ {
		if quote != 0 {
			if text[pos] == quote {
				quote = 0
			}
			continue
		}
		if text[pos] == '"' || text[pos] == '\'' {
			quote = text[pos]
			continue
		}
		if text[pos] == ':' && text[pos+1] == ':' {
			attrs := text[nameEnd:pos]
			if attrs != "" && !strings.ContainsRune(" \t\r\n", rune(attrs[0])) {
				return ToolMarkupTag{}, false
			}
			return ToolMarkupTag{Start: start, End: pos + 1, NameStart: nameStart, NameEnd: nameEnd, Name: name, Closing: closing, DSMLLike: true, ColonStyle: true}, true
		}
	}
	return ToolMarkupTag{}, false
}

func isPartialColonToolMarkupTagPrefix(text string) bool {
	if text == ":" || text == "::" || text == "::/" {
		return true
	}
	if !strings.HasPrefix(text, "::") {
		return false
	}
	if _, ok := scanColonToolMarkupTagAt(text, 0); ok {
		return false
	}
	rest := strings.ToLower(strings.TrimPrefix(text[2:], "/"))
	for _, name := range colonToolNames {
		if strings.HasPrefix(name.raw, rest) || strings.HasPrefix(rest, name.raw+" ") || strings.HasPrefix(rest, name.raw+":") {
			return true
		}
	}
	return false
}

func findColonRawClose(text string, from int) (int, bool) {
	for i := from; i+1 < len(text); i++ {
		if text[i:i+2] != "]]" {
			continue
		}
		if strings.HasPrefix(text[i:], "]] ]]") {
			i += 4
			continue
		}
		return i, true
	}
	return -1, false
}

func isColonRawStart(text string, pos int) bool {
	if pos < 0 || pos+2 > len(text) || text[pos:pos+2] != "[[" {
		return false
	}
	start := strings.LastIndex(strings.ToLower(text[:pos]), "::param")
	if start < 0 {
		return false
	}
	tag, ok := scanColonToolMarkupTagAt(text, start)
	return ok && tag.Name == "parameter" && !tag.Closing && tag.End < pos && strings.TrimSpace(text[tag.End+1:pos]) == ""
}

func rewriteColonRawValues(text string) string {
	if !strings.Contains(text, "[[") {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); {
		if isColonRawStart(text, i) {
			end, ok := findColonRawClose(text, i+2)
			if !ok {
				out.WriteString(text[i:])
				break
			}
			value := strings.ReplaceAll(text[i+2:end], "]] ]]", "]]")
			value = strings.ReplaceAll(value, "]]>", "]]]]><![CDATA[>")
			out.WriteString("<![CDATA[" + value + "]]>")
			i = end + 2
			continue
		}
		next, advanced, blocked := skipXMLIgnoredSection(text, i)
		if blocked {
			out.WriteString(text[i:])
			break
		}
		if advanced {
			out.WriteString(text[i:next])
			i = next
			continue
		}
		out.WriteByte(text[i])
		i++
	}
	return out.String()
}
