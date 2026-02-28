package tui

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/justin/glamdring/pkg/api"
)

// exportMarkdown converts conversation messages to a markdown document.
func exportMarkdown(msgs []api.RequestMessage) string {
	var b strings.Builder

	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			b.WriteString("## User\n\n")
		case "assistant":
			b.WriteString("## Assistant\n\n")
		}

		blocks := parseContentBlocks(msg.Content)
		for _, block := range blocks {
			switch block.Type {
			case "text", "":
				text := block.Text
				if text == "" {
					// Simple string content.
					if s, ok := msg.Content.(string); ok {
						text = s
					}
				}
				if text != "" {
					b.WriteString(text)
					b.WriteString("\n\n")
				}

			case "thinking":
				b.WriteString("<details>\n<summary>Thinking</summary>\n\n")
				b.WriteString(block.Thinking)
				b.WriteString("\n\n</details>\n\n")

			case "tool_use":
				b.WriteString(fmt.Sprintf("**Tool: %s**\n\n", block.Name))
				inputJSON, _ := json.MarshalIndent(json.RawMessage(block.Input), "", "  ")
				b.WriteString("```json\n")
				b.WriteString(string(inputJSON))
				b.WriteString("\n```\n\n")

			case "tool_result":
				if block.IsError {
					b.WriteString("**Tool Error:**\n\n")
				} else {
					b.WriteString("**Tool Result:**\n\n")
				}
				b.WriteString("```\n")
				b.WriteString(block.Content)
				b.WriteString("\n```\n\n")
			}
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// exportHTML converts conversation messages to a self-contained HTML page.
func exportHTML(msgs []api.RequestMessage) string {
	var body strings.Builder

	for _, msg := range msgs {
		role := msg.Role
		roleClass := role

		body.WriteString(fmt.Sprintf(`<div class="message %s">`, roleClass))
		body.WriteString(fmt.Sprintf(`<h2>%s</h2>`, strings.Title(role)))

		blocks := parseContentBlocks(msg.Content)
		for _, block := range blocks {
			switch block.Type {
			case "text", "":
				text := block.Text
				if text == "" {
					if s, ok := msg.Content.(string); ok {
						text = s
					}
				}
				if text != "" {
					body.WriteString(fmt.Sprintf(`<div class="text">%s</div>`, html.EscapeString(text)))
				}

			case "thinking":
				body.WriteString(`<details class="thinking"><summary>Thinking</summary>`)
				body.WriteString(fmt.Sprintf(`<pre>%s</pre>`, html.EscapeString(block.Thinking)))
				body.WriteString(`</details>`)

			case "tool_use":
				body.WriteString(fmt.Sprintf(`<div class="tool-call"><strong>Tool: %s</strong>`, html.EscapeString(block.Name)))
				inputJSON, _ := json.MarshalIndent(json.RawMessage(block.Input), "", "  ")
				body.WriteString(fmt.Sprintf(`<pre>%s</pre></div>`, html.EscapeString(string(inputJSON))))

			case "tool_result":
				cls := "tool-result"
				if block.IsError {
					cls = "tool-error"
				}
				body.WriteString(fmt.Sprintf(`<div class="%s"><pre>%s</pre></div>`, cls, html.EscapeString(block.Content)))
			}
		}

		body.WriteString(`</div>`)
	}

	return fmt.Sprintf(htmlTemplate, body.String())
}

// parseContentBlocks extracts ContentBlock slices from a RequestMessage.Content,
// which can be either a string or []ContentBlock (serialized as []any).
func parseContentBlocks(content any) []api.ContentBlock {
	switch v := content.(type) {
	case string:
		return []api.ContentBlock{{Type: "text", Text: v}}

	case []api.ContentBlock:
		return v

	case []any:
		// Content from JSON unmarshaling comes as []any of map[string]any.
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var blocks []api.ContentBlock
		if err := json.Unmarshal(data, &blocks); err != nil {
			return nil
		}
		return blocks

	default:
		// Try marshaling directly in case it's a typed slice.
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var blocks []api.ContentBlock
		if err := json.Unmarshal(data, &blocks); err != nil {
			return nil
		}
		return blocks
	}
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Glamdring Conversation Export</title>
<style>
:root {
  --bg: #1a1612;
  --fg: #d4be98;
  --fg-bright: #ebdbb2;
  --fg-dim: #7c6f64;
  --surface0: #282420;
  --surface1: #32302f;
  --amber: #e78a4e;
  --gold: #d8a657;
  --sage: #a9b665;
  --rust: #ea6962;
  --lavender: #d3869b;
  --teal: #89b482;
  --sky: #7daea3;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  background: var(--bg);
  color: var(--fg);
  font-family: "JetBrains Mono", "Fira Code", "Cascadia Code", monospace;
  font-size: 14px;
  line-height: 1.6;
  max-width: 900px;
  margin: 0 auto;
  padding: 2rem;
}
h2 {
  font-size: 0.9rem;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  margin-bottom: 0.5rem;
  padding-bottom: 0.25rem;
  border-bottom: 1px solid var(--surface1);
}
.message { margin-bottom: 2rem; }
.user h2 { color: var(--sky); }
.assistant h2 { color: var(--amber); }
.text {
  white-space: pre-wrap;
  word-wrap: break-word;
  margin-bottom: 1rem;
}
pre {
  background: var(--surface0);
  border: 1px solid var(--surface1);
  border-radius: 4px;
  padding: 1rem;
  overflow-x: auto;
  font-size: 13px;
  margin: 0.5rem 0 1rem;
}
.tool-call { margin: 0.5rem 0; }
.tool-call strong { color: var(--sage); }
.tool-result pre { border-left: 3px solid var(--sage); }
.tool-error pre { border-left: 3px solid var(--rust); color: var(--rust); }
details.thinking {
  margin: 0.5rem 0;
  border-left: 2px solid var(--lavender);
  padding-left: 1rem;
}
details.thinking summary {
  color: var(--lavender);
  cursor: pointer;
  font-style: italic;
}
details.thinking pre {
  color: var(--lavender);
  font-style: italic;
}
</style>
</head>
<body>
%s
</body>
</html>
`
