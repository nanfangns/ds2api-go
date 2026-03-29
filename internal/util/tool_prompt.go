package util

// BuildToolCallInstructions generates the unified tool-calling instruction block
// used by all adapters (OpenAI, Claude, Gemini). It uses attention-optimized
// structure: rules → negative examples → positive examples → anchor.
//
// The toolNames slice should contain the actual tool names available in the
// current request; the function picks real names for examples.
func BuildToolCallInstructions(toolNames []string) string {
	// Pick real tool names for examples; fall back to generic names.
	ex1 := "read_file"
	ex2 := "write_to_file"
	ex3 := "ask_followup_question"
	used := map[string]bool{}
	for _, n := range toolNames {
		switch {
		// Read/query-type tools
		case !used["ex1"] && matchAny(n, "read_file", "list_files", "search_files", "Read", "Glob"):
			ex1 = n
			used["ex1"] = true
		// Write/execute-type tools
		case !used["ex2"] && matchAny(n, "write_to_file", "apply_diff", "execute_command", "Write", "Edit", "MultiEdit", "Bash"):
			ex2 = n
			used["ex2"] = true
		// Interactive/meta tools
		case !used["ex3"] && matchAny(n, "ask_followup_question", "attempt_completion", "update_todo_list", "Task"):
			ex3 = n
			used["ex3"] = true
		}
	}

	return `TOOL CALL FORMAT — FOLLOW EXACTLY:

When calling tools, emit ONLY raw XML. No text before, no text after, no markdown fences.

<tool_calls>
  <tool_call>
    <tool_name>TOOL_NAME_HERE</tool_name>
    <parameters>{"key":"value"}</parameters>
  </tool_call>
</tool_calls>

RULES:
1) Output ONLY the XML above when calling tools. Do NOT mix tool XML with regular text.
2) <parameters> MUST contain a strict JSON object. All JSON keys and strings use double quotes.
3) Multiple tools → multiple <tool_call> blocks inside ONE <tool_calls> root.
4) Do NOT wrap the XML in markdown code fences (no triple backticks).
5) After receiving a tool result, use it directly. Only call another tool if the result is insufficient.
6) If you want to say something AND call a tool, output text first, then the XML block on its own.

❌ WRONG — Do NOT do these:
Wrong 1 — mixed text and XML:
  I'll read the file for you. <tool_calls><tool_call>...
Wrong 2 — describing tool calls in text:
  [调用 Bash] {"command": "ls"}
Wrong 3 — missing <tool_calls> wrapper:
  <tool_call><tool_name>` + ex1 + `</tool_name><parameters>{}</parameters></tool_call>

✅ CORRECT EXAMPLES:

Example A — Single tool:
<tool_calls>
  <tool_call>
    <tool_name>` + ex1 + `</tool_name>
    <parameters>{"path":"src/main.go"}</parameters>
  </tool_call>
</tool_calls>

Example B — Two tools in parallel:
<tool_calls>
  <tool_call>
    <tool_name>` + ex1 + `</tool_name>
    <parameters>{"path":"config.json"}</parameters>
  </tool_call>
  <tool_call>
    <tool_name>` + ex2 + `</tool_name>
    <parameters>{"path":"output.txt","content":"Hello world"}</parameters>
  </tool_call>
</tool_calls>

Example C — Tool with complex nested JSON parameters:
<tool_calls>
  <tool_call>
    <tool_name>` + ex3 + `</tool_name>
    <parameters>{"question":"Which approach do you prefer?","follow_up":[{"text":"Option A"},{"text":"Option B"}]}</parameters>
  </tool_call>
</tool_calls>

Remember: Output ONLY the <tool_calls>...</tool_calls> XML block when calling tools.`
}

func matchAny(name string, candidates ...string) bool {
	for _, c := range candidates {
		if name == c {
			return true
		}
	}
	return false
}
