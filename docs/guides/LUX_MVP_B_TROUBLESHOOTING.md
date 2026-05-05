# LUX MVP-B Troubleshooting — Automated Lead Follow-Up Sequence

## Quick Checks First

Before digging into individual nodes:

1. Is the workflow **activated**? (Toggle in top-right of n8n workflow editor)
2. Are all credentials assigned? (Open each HTTP Request node → check the credential dropdown)
3. Is the Airtable table named exactly `Leads`? (Case-sensitive)
4. Is the Slack bot invited to `#new-leads`?

---

## Webhook Not Receiving Payloads

**Symptom:** Pipeline never triggers; `curl` returns connection refused or 404.

**Checks:**
- Confirm the workflow is activated — the webhook only listens when active
- Verify the webhook path: `POST /webhook/lux-lead-followup`
- If self-hosted, ensure n8n is reachable from the source (firewall, reverse proxy)
- Test with a direct curl from the server: `curl -X POST http://localhost:5678/webhook/lux-lead-followup -H "Content-Type: application/json" -d '{"name":"test"}'`

---

## Classification Returns Wrong `project_category`

**Symptom:** A commercial lead routes to the residential sequence (or vice versa).

**Checks:**
- Open the **Parse Classification** node output in the last execution — check the raw `project_category` value
- If the input payload's `project_type` field is ambiguous (e.g., "renovation"), Claude may classify it differently than expected
- The IF node routes `commercial_renovation` and `multi_family` to commercial — all other values go residential
- Fix: ensure the input payload's `project_type` matches one of the five expected values: `custom_home`, `commercial_renovation`, `addition_or_remodel`, `multi_family`, or leave blank for Claude to infer from `message`

**Prompt fix:** If Claude consistently misclassifies a project type, edit `docs/mvps/mvp-b/prompts/user_classify.txt` and add a clarifying example. Sync the change to the **Read Classify Prompt** Code node in the workflow.

---

## Email Quality Issues

### Email 1 Opens with a Generic Sentence

**Symptom:** Email 1 doesn't reference the lead's specific words. Feels like a template.

**Cause:** The `message` field in the payload was empty, very short, or too generic for Claude to extract a `key_detail`.

**Fix:** Ensure the payload's `message` field contains what the lead actually said. Check the **Parse Classification** node output — if `key_detail` is `null`, Claude had nothing to work with.

### Subject Lines Are Too Generic

**Symptom:** Subject says something like "Your project" or "Construction inquiry".

**Cause:** The classification extracted a vague `key_detail`, or the prompt instructions weren't followed.

**Fix:** Check `key_detail` in the Airtable record. If it's generic, the issue is in the input. If it's specific but the subject line ignored it, file a prompt improvement — edit `user_sequence_commercial.txt` or `user_sequence_residential.txt`.

### Email 2 or 3 Sounds Pushy

**Symptom:** Email 2 opens with "Just checking in" or Email 3 feels like a last-ditch attempt.

**Cause:** Claude occasionally drifts from brand voice on longer outputs.

**Fix:** Edit the failing email in Airtable before sending. For recurring drift, tighten the relevant prompt file and resync to the n8n Code node.

---

## Airtable Record Not Created

**Symptom:** Pipeline completes, Slack fires, but no record appears in Airtable.

**Checks:**
1. Open the **Airtable: Create Lead Record** node in the last execution — check for error output
2. Common errors:
   - `NOT_FOUND`: Base ID is wrong or the table is not named `Leads`
   - `INVALID_PERMISSIONS`: Personal access token lacks `data.records:write` scope for the base
   - `INVALID_VALUE_FOR_COLUMN`: A field value doesn't match the field type (e.g., a string in a checkbox field)
3. Verify the Base ID: go to your Airtable base → the URL is `https://airtable.com/BASE_ID/TABLE_ID/...`

---

## Airtable Field Mapping Errors

**Symptom:** Record creates but some fields are blank or show wrong values.

**Cause:** Field names in the Airtable node don't exactly match your table's field names.

**Fix:**
1. Open the **Airtable: Create Lead Record** node
2. Compare each mapped field name against your Airtable table (exact case, exact spacing)
3. "Urgency Signal" must be a Checkbox field — passing a string will fail silently in some versions

---

## Slack Message Not Posting

**Symptom:** Pipeline completes, Airtable record created, but nothing appears in `#new-leads`.

**Checks:**
1. Open the **Post to #new-leads** node in the last execution — check for error output
2. `channel_not_found`: Bot is not in the channel. Run `/invite @your-bot-name` in `#new-leads`
3. `invalid_auth`: Slack credential expired or revoked — reconnect in n8n Credentials
4. Channel name vs. ID: if the channel name has changed, switch to the channel ID (Settings → Copy link → extract the ID from the URL)

---

## Anthropic API Errors

**Symptom:** Any HTTP Request node returns a 4xx or 5xx error from the Anthropic API.

| Error | Cause | Fix |
|---|---|---|
| 401 Unauthorized | Invalid API key | Verify `x-api-key` credential value |
| 429 Too Many Requests | Rate limit hit | Add a Wait node before the failing HTTP Request (1–2 seconds) |
| 400 Bad Request | Malformed JSON body | Open the node, check the `jsonBody` field for syntax errors from expression interpolation |
| 529 Overloaded | Anthropic API load | Retry after a few seconds; add error handling if this recurs |

---

## JSON Parse Failures

**Symptom:** A Code node (`Parse Classification`, `Parse Sequence JSON`, `Parse Slack JSON`) throws an error like `JSON.parse: unexpected token`.

**Cause:** Claude occasionally returns markdown fences (` ```json `) or adds preamble text before the JSON object despite explicit instructions.

**Check:** Open the failing node's execution, look at the input — find the raw `content[0].text` value. The Code nodes already strip markdown fences as a fallback. If they're still failing, the response format is further degraded.

**Fix:**
1. Add a stricter instruction to the relevant prompt: "Return ONLY the raw JSON object. No text before or after. No markdown."
2. For persistent failures, add a more robust strip before `JSON.parse`:
```javascript
const cleaned = raw.replace(/^[^{[]*/, '').replace(/[^}\]]*$/, '');
parsed = JSON.parse(cleaned);
```

---

## IF Node Routes All Leads to Residential

**Symptom:** Every lead goes to the residential sequence regardless of `project_category`.

**Cause:** The IF node condition uses a regex match on `project_category`. If the value has unexpected casing or whitespace, the regex won't match.

**Fix:**
1. Check the **Build Context** node output — look at the exact `project_category` value
2. The regex is: `^(commercial_renovation|multi_family)$`
3. If values come in with different casing (e.g., `Commercial_Renovation`), add `.toLowerCase()` to the Build Context Code node where `project_category` is assigned

---

## Merge Node Passes Empty Data

**Symptom:** **Parse Sequence JSON** fails because input is empty after the Merge node.

**Cause:** The Merge node is configured for `passThrough` / `single` mode — it passes the first item it receives. If neither branch completed, or if both completed and the second overwrote the first, data can be lost.

**Fix:**
1. Confirm only one branch executes per run (the IF node should guarantee this)
2. If both branches fire (misconfigured IF), the Merge will try to combine them — verify the IF node condition is correct
3. In n8n, the Merge node input indices matter: Commercial → input 0, Residential → input 1. Verify the connections in the workflow match.

---

## Checking Execution History

1. In n8n, go to **Executions** (left sidebar)
2. Find the failed run — executions show success/failure per node
3. Click any node to see its input and output data at that step
4. This is the fastest way to isolate where the pipeline broke

---

*See [LUX_MVP_B_SETUP.md](LUX_MVP_B_SETUP.md) for initial configuration.*
*See [LUX_MVP_B_USER_GUIDE.md](LUX_MVP_B_USER_GUIDE.md) for day-to-day usage.*
*See [N8N_GUIDE.md § 9 — Operations Reference](N8N_GUIDE.md) for n8n-level troubleshooting.*
