# MVP A Loom Draft

## Goal

Show how LUX turns project data into a client-ready status email, sends the draft to Slack, and keeps the PM in control.

## Audience

Owners, PMs, and operators who want a working AI workflow, not a prompt demo.

## Recommended Screen Flow

1. `docs/mvps/mvp-a/README.md`
2. `docs/mvps/mvp-a/payload.json`
3. `docs/mvps/mvp-a/n8n_workflow.json` in n8n
4. Recent execution showing Build Context, email generation, Slack copy, and Slack output
5. `docs/mvps/mvp-a/payload_alt.json`

## Timed Talk Track

### 0:00-0:30

"This MVP writes client status updates for LUX. It takes a structured project payload, drafts the email, and drops that draft into Slack for PM review. Nothing goes to the client unless someone approves it."

### 0:30-1:00

"It solves a pretty repetitive PM task. Status emails usually mean pulling details from JobTread, memory, and site notes, then writing the same kind of message again. This workflow turns that into a fast first draft and keeps the tone consistent."

### 1:00-1:40

"Here’s the sample payload. It includes phase, percent complete, schedule detail, milestones, budget context, and open items. The key thing is that it’s structured. Claude isn’t guessing from a messy paragraph. It’s working from a clean project snapshot."

"One detail that matters here is client action item detection. The workflow scans open items for words like approve, select, confirm, and decide. That signal lets the Slack message show a little urgency when the client is the blocker."

### 1:40-2:30

"In n8n, the flow is pretty straightforward. The webhook receives the payload. A Code node builds context. Another node loads prompts from the repo. Claude Sonnet writes the subject and body. The workflow parses that JSON, then a lighter Claude call writes the Slack note. Slack gets a review-ready draft with approve and edit options."

"That split matters. The email is for the homeowner. The Slack note is for the PM team. Those are different jobs, so the workflow handles them separately."

### 2:30-3:15

"Now I’d show the output. The subject is specific. The body stays short, direct, and safe for the client. It covers what changed, what comes next, and whether the client owes a decision. It doesn’t dump internal task noise into the message."

"The Slack post reads more like a teammate handing you a draft than a robot dumping text. That makes review faster."

### 3:15-4:10

"The second payload is the real test. It includes a supplier delay and a budget overrun above the threshold. The workflow runs through the same nodes, but the prompt rules change the output. Minor variance stays out. Material variance gets stated plainly and with context."

"That rule set is what makes this useful. You do not want AI improvising on budget communication. The prompt and the structured payload set clear guardrails around what gets said and what stays internal."

### 4:10-4:40

"In one sentence: Claude drafts the update, but the PM still owns the message. That gives you speed without giving up judgment."

## Key Points To Land

- The workflow is built around review, not blind automation.
- Structured payloads reduce hallucination risk and improve consistency.
- Budget variance and client action item rules are explicit, not implied.
- Separate AI calls handle external communication and internal notification differently.

## Close

"In production, the first win is time saved. The bigger win is consistency. Every client gets a clear, professional update, no matter which PM sends it."
