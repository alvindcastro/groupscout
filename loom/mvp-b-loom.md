# MVP B Loom Draft

## Goal

Show how an inbound lead gets classified, routed into the right email sequence, stored in Airtable, and surfaced in Slack without auto-send.

## Audience

Sales leaders, operators, and founders who want faster follow-up without weaker copy.

## Recommended Screen Flow

1. `docs/mvps/mvp-b/README.md`
2. `docs/mvps/mvp-b/payload.json`
3. `docs/mvps/mvp-b/payload_alt.json`
4. `docs/mvps/mvp-b/n8n_workflow.json` in n8n
5. Airtable record created by the workflow
6. Slack notification output

## Timed Talk Track

### 0:00-0:30

"This MVP writes lead follow-up for LUX. It takes an inbound lead, classifies it, generates a three-email sequence, stores the drafts in Airtable, and pings sales in Slack."

### 0:30-1:00

"The key design choice is simple: it classifies before it writes. A lot of AI demos jump straight from form submission to email copy. This workflow adds a reasoning step first, so the writing matches lead quality and project category."

### 1:00-1:45

"Here are the two sample payloads. The first is Marcus Webb, a commercial lead with a clear budget, specific scope, and near-term timing. The second is Jennifer Okafor, a residential lead with a softer timeline and a different buying posture."

"Those differences aren’t cosmetic. They should trigger different sales behavior. Commercial renovation and custom home follow-up should not sound the same, and this workflow is built around that."

### 1:45-2:35

"In the workflow, the webhook receives the lead, the prompt files load from the repo, and Claude Haiku classifies the lead first. It returns lead tier, project category, key detail, and urgency signal. A Code node builds context and sends the run through an IF node."

"If the project category is commercial renovation or multi-family, the workflow takes the commercial branch. Everything else goes to residential. Then the outputs merge, the three emails are parsed, Airtable gets a new lead record, and a final AI call writes the Slack note."

### 2:35-3:20

"The value shows up in the output. Email 1 mirrors the lead’s own language. If Marcus says he has two tenant spaces, that phrase shows up in the draft. Email 2 adds a commercial-specific insight like phased permitting or tenant coordination. That would feel wrong for a homeowner lead."

"That’s the difference between personalization and templating. It isn’t just name-swapping. It’s choosing the right sales conversation."

### 3:20-4:05

"Airtable becomes the review layer. Sales can see the original message, the classification fields, and all three drafts in one place. Nothing auto-sends. It’s basically a staging area for review, editing, and ownership."

"Slack closes the loop with a short internal note. It flags high-tier leads when needed and asks who owns follow-up. That makes the handoff explicit."

### 4:05-4:40

"Then I’d trigger the residential payload and show the branch switch. Same system, same webhook, different sequence style. That shows why routing logic works better than one giant prompt."

## Key Points To Land

- The workflow classifies first, then writes.
- Commercial and residential leads use different sequence prompts.
- Airtable is the review layer, not just storage.
- Slack creates accountability by surfacing ownership immediately.

## Close

"The short version of MVP B is this: it turns inbound lead capture into a structured sales workflow. AI handles the first draft, but the team still controls qualification, edits, and send timing."
