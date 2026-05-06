# MVP-C Troubleshooting — LUX LinkedIn Post Pipeline

---

## Slack Shows `json { ... }` Instead of a Normal Review Message

**Symptom:** Slack shows a raw blob like `json { "slack_message": "..." }` or the Slack copy includes markdown fences.

**Cause:** Claude returned valid content, but not as a clean bare JSON object. Common variants:
- prefixed text like `json { ... }`
- fenced output like ```` ```json ... ``` ````
- extra text before or after the JSON object

**Current workflow behavior:** The latest `docs/mvps/mvp-c/n8n_workflow.json` already handles this in both `Extract Post` and `Build Slack Message` using tolerant JSON extraction.

**Fix if your live workflow still fails:** Re-import the latest workflow JSON or paste the updated code from the current export into both Code nodes:
- `Extract Post`
- `Build Slack Message`

The parser now:
- tries `JSON.parse()` directly
- strips fenced JSON if present
- falls back to slicing from the first `{` to the last `}`

**What to verify in the execution:**
- `Claude: Slack Notification Copy` may still show prefixed or fenced JSON in `content[0].text`
- `Build Slack Message` must normalize that into a plain `notification_text`
- `Slack: Post to #content-review` should post the normalized text, not the raw JSON

---

## Claude Returns Non-JSON or Wraps in Markdown Fences

**Symptom:** The `Extract Post` Code node fails with a JSON parse error, or the `post` field falls back to raw text.

**Cause:** The model occasionally wraps output in ` ```json ``` ` fences or adds a preamble sentence despite the prompt instruction.

**Fix:** The current export already includes the tolerant parser below in `Extract Post` and `Build Slack Message`:

```js
const parseJSON = (value) => {
  const trimmed = String(value || '').trim();

  try {
    return JSON.parse(trimmed);
  } catch (err) {
    const fenced = trimmed.match(/```(?:json)?\n?([\s\S]*?)\n?```/i);
    if (fenced) return JSON.parse(fenced[1]);

    const start = trimmed.indexOf('{');
    const end = trimmed.lastIndexOf('}');
    if (start !== -1 && end !== -1 && end >= start) {
      return JSON.parse(trimmed.slice(start, end + 1));
    }

    throw err;
  }
};
```

If this happens repeatedly, add `"Return JSON only. No markdown fences."` as the last line of the failing prompt file in `docs/mvps/mvp-c/prompts/`.

---

## Slack Message Does Not Appear in #content-review

**Check 1 — Slack credential is correct.**
In n8n, open the Slack node and verify it uses the Slack bot credential assigned to the workspace where `#content-review` exists.

**Check 2 — n8n execution log.**
Go to **Executions** in n8n. Find the latest run. Click into it and look at the Slack node output. If it shows `error: channel_not_found`, the bot is either posting to the wrong workspace or is not in the channel.

**Check 3 — Slack app permissions.**
The bot must be invited to `#content-review`:
```
/invite @your-bot-name
```

Also confirm the token has `chat:write`.

---

## How to Inspect a Failed Execution

Use this order when debugging:

1. Open **Executions** in n8n.
2. Click the failed or suspicious run.
3. Inspect `Load Prompts` first.
4. Inspect `Route by Type`.
5. Inspect the Claude node that actually ran.
6. Inspect `Extract Post`.
7. Inspect `Claude: Slack Notification Copy`.
8. Inspect `Build Slack Message`.
9. Inspect `Slack: Post to #content-review`.

**Expected outputs by node:**

| Node | Healthy output |
| --- | --- |
| `Load Prompts` | All prompt strings loaded under `prompts.*` |
| `Route by Type` | Milestone payload goes true branch; podcast payload goes false branch |
| `Claude: Milestone Post` or `Claude: Podcast Post` | `content[0].text` contains a JSON object with `post` |
| `Extract Post` | Clean `post`, `content_type`, `about`, `post_preview` |
| `Claude: Slack Notification Copy` | `content[0].text` contains a JSON object with `slack_message` |
| `Build Slack Message` | Clean `notification_text` |
| `Slack: Post to #content-review` | Success response from Slack |

---

## Wrong Branch Fired (Milestone Payload Went to Podcast Branch or Vice Versa)

**Symptom:** The output post reads like a podcast post but you sent a milestone payload, or vice versa.

**Cause:** The IF node condition compares `$json.type` — if your payload has a typo in the `type` field, the false branch fires (podcast).

**Fix:** Verify the payload:
```json
{ "type": "project_milestone" }  // exact string, no whitespace
{ "type": "podcast_episode" }    // exact string, no whitespace
```

Any value other than `project_milestone` goes to the podcast branch. Check for trailing spaces or case differences.

---

## Anthropic API Returns 401 Unauthorized

**Symptom:** Both Claude HTTP Request nodes fail with `401 Unauthorized`.

**Fix:** The `x-api-key` header is missing or wrong. In the n8n credential:
- Header Name must be exactly `x-api-key` (lowercase, hyphenated)
- Value must be the full API key including the `sk-ant-` prefix

Also confirm the key is active in the Anthropic Console and has not hit its spending limit.

---

## Anthropic API Returns 529 Overloaded

**Symptom:** Occasional failures during high-traffic periods. The error body contains `overloaded_error`.

**Fix:** Add a **Wait** node before the Claude HTTP Request nodes set to 5–10 seconds, or enable n8n's built-in retry (3 attempts, 2s delay) on the HTTP Request nodes:

1. Open the node.
2. Go to **Settings** tab.
3. Enable **Continue on Fail** + set **Retry on Fail** to 3.

---

## Post Exceeds 150 Words

**Symptom:** The generated post is longer than the LUX brand spec allows.

**Cause:** The model can occasionally exceed the word limit if the input payload is information-dense.

**Fix options:**
1. Trim the input payload — shorten `detail` or reduce `key_takeaways` to 2 items.
2. Add a word count check in the `Extract Post` Code node and log a warning if `post.split(' ').length > 150`.
3. Append to the user prompt: `"The post MUST be under 150 words. Count carefully."` — this usually helps.

---

## Slack Notification Copy Feels Robotic

**Symptom:** Prompt 3 (Slack copy) outputs something generic like "A LinkedIn post draft is ready. Please review."

**Cause:** The `post_context_json` passed to Prompt 3 was empty or missing the `about` field.

**Fix:** Confirm the `Extract Post` Code node is correctly building the context object:
```js
{
  content_type: inputData.type,
  about: inputData.type === 'project_milestone'
    ? `${inputData.milestone} — ${inputData.project_name}`
    : `Episode ${inputData.episode} with ${inputData.guest} on ${inputData.topic}`,
  post_preview: post.split('\n')[0]
}
```
If `inputData.milestone` is undefined, `about` becomes `undefined — undefined` and Claude has nothing to work with.

---

## n8n Workflow Import Fails

**Symptom:** Importing `n8n_workflow.json` shows an error about unsupported node types or version mismatch.

**Fix:**
- Confirm n8n version is **1.0+** (`npx n8n --version`).
- If a node type like `n8n-nodes-base.merge` shows as unknown, update n8n: `npm update -g n8n` or pull the latest Docker image.
- As a fallback, rebuild the workflow manually using the node table in `docs/mvps/mvp-c.md` (Build Path section).

---

## Buffer Step Not Firing After Slack Button Click

**Symptom:** Clicking "Schedule via Buffer" in Slack does nothing.

**Cause:** The Buffer step is optional and not wired in the base workflow. The exported workflow prepares a `blocks` payload in `Build Slack Message`, but the current Slack node posts `notification_text` only. Interactive scheduling requires a separate Slack interaction workflow.

**Fix:** Create a second n8n workflow:
1. **Webhook** node — receives Slack interactive component callbacks.
2. **IF** node — check `$json.payload.actions[0].value === 'schedule'`.
3. **Buffer HTTP Request** node — POST to Buffer's `POST /2/links/shares` endpoint with the post text.

The base MVP-C workflow handles draft generation only. Scheduling is a separate integration.

---

## FAQ

**Can I run both a milestone and an episode payload at the same time?**
Yes. n8n executes each webhook trigger as an independent run. Concurrent executions are supported. Both Slack messages will land in `#content-review` in the order they complete (usually within 3–5 seconds of each other).

**Can I change the model?**
Yes. Open the Claude HTTP Request nodes and change the `model` field in the JSON body. Use `claude-haiku-4-5-20251001` for faster/cheaper drafts at the cost of voice consistency. Use `claude-opus-4-6` for the highest fidelity.

**Can I A/B test prompts?**
Yes. Duplicate the Claude HTTP Request node, swap in a modified prompt, and wire both to a **Compare** Code node that returns both drafts as separate Slack messages.

**How do I re-run a failed execution?**
In n8n, go to **Executions**, find the failed run, and click **Retry**. This replays the original webhook payload through the workflow.
