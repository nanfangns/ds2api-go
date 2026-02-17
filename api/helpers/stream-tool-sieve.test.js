'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const {
  extractToolNames,
  createToolSieveState,
  processToolSieveChunk,
  flushToolSieve,
  parseToolCalls,
} = require('./stream-tool-sieve');

function runSieve(chunks, toolNames) {
  const state = createToolSieveState();
  const events = [];
  for (const chunk of chunks) {
    events.push(...processToolSieveChunk(state, chunk, toolNames));
  }
  events.push(...flushToolSieve(state, toolNames));
  return events;
}

function collectText(events) {
  return events
    .filter((evt) => evt.type === 'text' && evt.text)
    .map((evt) => evt.text)
    .join('');
}

test('extractToolNames keeps tool mode enabled with unknown fallback', () => {
  const names = extractToolNames([
    { function: { description: 'no name tool' } },
    { function: { name: ' read_file ' } },
    {},
  ]);
  assert.deepEqual(names, ['unknown', 'read_file', 'unknown']);
});

test('parseToolCalls keeps non-object argument strings as _raw (Go parity)', () => {
  const payload = JSON.stringify({
    tool_calls: [
      { name: 'read_file', input: '123' },
      { name: 'list_dir', input: '[1,2,3]' },
    ],
  });
  const calls = parseToolCalls(payload, ['read_file', 'list_dir']);
  assert.deepEqual(calls, [
    { name: 'read_file', input: { _raw: '123' } },
    { name: 'list_dir', input: { _raw: '[1,2,3]' } },
  ]);
});

test('parseToolCalls still intercepts unknown schema names to avoid leaks', () => {
  const payload = JSON.stringify({
    tool_calls: [{ name: 'not_in_schema', input: { q: 'go' } }],
  });
  const calls = parseToolCalls(payload, ['search']);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].name, 'not_in_schema');
});

test('parseToolCalls supports fenced json and function.arguments string payload', () => {
  const text = [
    'I will call a tool now.',
    '```json',
    '{"tool_calls":[{"function":{"name":"read_file","arguments":"{\\"path\\":\\"README.md\\"}"}}]}',
    '```',
  ].join('\n');
  const calls = parseToolCalls(text, ['read_file']);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].name, 'read_file');
  assert.deepEqual(calls[0].input, { path: 'README.md' });
});

test('sieve emits tool_calls and does not leak suspicious prefix on late key convergence', () => {
  const events = runSieve(
    [
      '{"',
      'tool_calls":[{"name":"read_file","input":{"path":"README.MD"}}]}',
      '后置正文C。',
    ],
    ['read_file'],
  );
  const leakedText = collectText(events);
  const hasToolCall = events.some((evt) => evt.type === 'tool_calls' && Array.isArray(evt.calls) && evt.calls.length > 0);
  assert.equal(hasToolCall, true);
  assert.equal(leakedText.includes('{'), false);
  assert.equal(leakedText.toLowerCase().includes('tool_calls'), false);
  assert.equal(leakedText.includes('后置正文C。'), true);
});

test('sieve drops invalid tool json body while preserving surrounding text', () => {
  const events = runSieve(
    [
      '前置正文D。',
      "{'tool_calls':[{'name':'read_file','input':{'path':'README.MD'}}]}",
      '后置正文E。',
    ],
    ['read_file'],
  );
  const leakedText = collectText(events);
  const hasToolCall = events.some((evt) => evt.type === 'tool_calls');
  assert.equal(hasToolCall, false);
  assert.equal(leakedText.includes('前置正文D。'), true);
  assert.equal(leakedText.includes('后置正文E。'), true);
  assert.equal(leakedText.toLowerCase().includes('tool_calls'), false);
});

test('sieve suppresses incomplete captured tool json on stream finalize', () => {
  const events = runSieve(
    ['前置正文F。', '{"tool_calls":[{"name":"read_file"'],
    ['read_file'],
  );
  const leakedText = collectText(events);
  assert.equal(leakedText.includes('前置正文F。'), true);
  assert.equal(leakedText.toLowerCase().includes('tool_calls'), false);
  assert.equal(leakedText.includes('{'), false);
});

test('sieve keeps plain text intact in tool mode when no tool call appears', () => {
  const events = runSieve(
    ['你好，', '这是普通文本回复。', '请继续。'],
    ['read_file'],
  );
  const leakedText = collectText(events);
  const hasToolCall = events.some((evt) => evt.type === 'tool_calls');
  assert.equal(hasToolCall, false);
  assert.equal(leakedText, '你好，这是普通文本回复。请继续。');
});
