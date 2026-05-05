# LUX MVP-A Troubleshooting — AI Client Status Email Generator

## Quick Checks First

Before diving into individual symptoms, verify these baseline conditions:

1. The workflow is **Active** in n8n (toggle is on)
2. Both API credentials are attached to their nodes (Anthropic and Slack)
3. The bot is invited to `#client-updates-review` in Slack
4. The payload is valid JSON (use a linter if unsure)

---

## Webhook Returns 404

**Cause:** Workflow is not active, or the webhook path changed.

**Fix:**
- Toggle the workflow to Active
- Check the Webhook node — path should be `lux-status-email`
- Confirm you're using the correct n8n base URL (production vs. test)

Note: n8n has two webhook URLs — test (`/webhook-test/`) and production (`/webhook/`). The production URL only works when the workflow is active.

---

## Anthropic API Call Fails (HTTP 401)

**Cause:** Missing or invalid API key.

**Fix:**
- Open the **Generate Status Email** node
- Confirm the HTTP Header Auth credential is attached
- Verify the credential has Header Name `x-api-key` and a valid `sk-ant-...` value
- Check the same on **Generate Slack Copy**

---

## Anthropic API Call Fails (HTTP 429)

**Cause:** Rate limit hit.

**Fix:** Retry after 60 seconds. If this happens frequently, check your Anthropic usage dashboard. The pipeline makes two API calls per run — both against the same key.

---

## Parse Email JSON Node Fails

**Cause:** Claude returned output that wasn't clean JSON — usually markdown fences or preamble text.

**Symptoms:** n8n shows a JSON parse error in the **Parse Email JSON** node.

**Fix:** The Code node already handles markdown fence stripping. If it still fails:
1. Open the failed execution in n8n
2. Check the raw output from **Generate Status Email** — look at `content[0].text`
3. If Claude added explanation outside the JSON, update `prompts/user_status_email.txt` to reinforce "Return JSON only. No preamble, no markdown fences, no explanation."
4. Re-run the workflow — the **Load Prompts** node will read the updated file on the next execution

---

## Email Has Construction Jargon ("rough-in", "MEP", etc.)

**Cause:** Brand voice rules not applied, or model slipped.

**Fix:**
1. Check that the **Generate Status Email** node has the `system` parameter set from **Load Prompts** output
2. Open `prompts/system_brand_voice.txt` — confirm the forbidden terms list is present
3. Add the specific term to the "WHAT NEVER APPEARS" list if it's missing
4. Re-run the workflow so **Load Prompts** picks up the change

---

## Budget Variance Not Mentioned When It Should Be

**Scenario:** `budget_variance` is -31500 on a $480,000 project (6.6% over) but the email says nothing about it.

**Cause:** `budget_total` field was not included in the payload — the pipeline cannot calculate the percentage threshold without it.

**Fix:** Include `budget_total` in the payload whenever there is a meaningful variance. Without it, Claude cannot determine whether the threshold is crossed.

---

## Budget Variance Mentioned When It Shouldn't Be

**Scenario:** Minor variance (-4200 on a large project) appears in the email.

**Cause:** `budget_total` is missing — Claude can't calculate the % threshold and may default to mentioning any negative variance.

**Fix:** Always include `budget_total` in the payload.

---

## Client Action Item Not Surfaced in Email

**Scenario:** `open_items` includes "Client to select cabinet hardware" but the email doesn't call it out.

**Cause:** The item doesn't contain a keyword that triggers the `hasClientItems` flag in the Build Context node.

**Fix:**
- Check the **Build Context** Code node output in the failed execution — look at `has_client_action_items`
- If `false`, the keyword wasn't matched. Add a keyword to the detection list in the Code node:
  ```javascript
  const clientKeywords = ['client', 'owner', 'select', 'confirm', 'approve', 'decide', 'choose', 'your_new_keyword'];
  ```
- Alternatively, rephrase the `open_items` entry to include a matching keyword

---

## Internal Tasks Appearing in the Client Email

**Scenario:** "LUX to confirm revised steel delivery date" appears in the email body.

**Cause:** Claude misclassified an internal item as client-facing.

**Fix:**
1. Confirm the prompt rule is intact in `prompts/user_status_email.txt`:
   > "If open_items are internal LUX tasks: do not mention them"
2. If the item contains ambiguous language, make it more explicitly internal in the source data (prefix with "LUX to..." or "Internal:")

---

## Slack Message Not Posted

**Cause:** Bot not in channel, wrong channel name, or Slack credential error.

**Fix:**
1. Confirm the Slack bot is invited: `/invite @your-bot-name` in `#client-updates-review`
2. Check the **Post to Slack** node — channel ID vs. name mismatch is common. Use the channel ID (starts with `C`) rather than the name for reliability
3. Verify the Slack credential is active and the token hasn't expired

---

## Slack Message Sounds Like a Bot

**Symptom:** The notification copy is stiff or uses formal language instead of casual team tone.

**Fix:** Edit `prompts/user_slack_notify.txt`:
- Reinforce "casual internal tone — this is a team channel"
- Add examples of what casual sounds like vs. what to avoid
- Re-run the workflow so **Load Prompts** picks up the change

---

## Email Body Exceeds 200 Words

**Cause:** Claude didn't honour the length constraint.

**Fix:**
1. Add an explicit word count check to `prompts/user_status_email.txt`:
   > "Count your words before returning. If the body exceeds 200 words, cut the longest section first."
2. For persistent issues, add a post-processing check in the **Parse Email JSON** Code node that warns if `body.split(' ').length > 200`

---

## Full Execution Debug

To see all node inputs/outputs for a past run:

1. Go to n8n → **Executions**
2. Find the failed or suspect run
3. Click into it — each node shows its input and output data
4. The **Build Context** node output shows `project_json` and `notification_context_json` — verify these look right before the API calls

---

## Related Docs

- [LUX_MVP_A_SETUP.md](LUX_MVP_A_SETUP.md) — credential setup and workflow import
- [LUX_MVP_A_USER_GUIDE.md](LUX_MVP_A_USER_GUIDE.md) — day-to-day usage reference
- [N8N_GUIDE.md](N8N_GUIDE.md) — general n8n operations and credential management
