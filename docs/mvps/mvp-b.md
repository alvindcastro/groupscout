# MVP B — Automated Lead Follow-Up Sequence

## What It Does

Accepts an inbound lead payload (simulating a website contact form or CRM entry), calls Claude to generate a personalized 3-email follow-up sequence, logs all drafts to Airtable, and fires a Slack notification to the sales team.

---

## Problem It Solves

LUX sales follow-up depends on who picks up the lead and when. Response times vary, email quality varies, and sequences trail off. This pipeline ensures every lead gets a consistent, personalized sequence within seconds — drafted and staged, ready for a human to send.

---

## Input

Simulated inbound lead payload:

```json
{
  "name": "Marcus Webb",
  "company": "Webb Commercial Properties",
  "source": "website_contact_form",
  "project_type": "commercial_renovation",
  "budget_range": "250k-500k",
  "timeline": "Q3 2025",
  "message": "Looking to renovate two tenant spaces in our office building downtown. Want to discuss scope and timeline."
}
```

---

## Prompts

The pipeline uses four prompts. Prompts 1 and 2 are always used. Prompt 3 routes based on project type. Prompt 4 generates the Slack notification copy.

---

### Prompt 1 — Brand Voice System Prompt (applied to every call as `system`)

The persistent sales identity layer. Set as the `system` parameter on every Anthropic API call.

```
You are the sales voice of LUX, a high-end general contractor that builds
custom homes and commercial projects for discerning clients.

LUX is selective. They do not chase every lead. They qualify well and work
with clients who value craftsmanship, clear communication, and a builder
who treats their project like it matters — because it does.

THE LUX SALES VOICE

TONE
- Confident and consultative. LUX leads with expertise, not availability.
- Never desperate, never pushy. If a lead is not ready, LUX leaves the door
  open with zero pressure.
- Warm but not sycophantic. No "Great question!" energy. No hollow enthusiasm.
- Peer-to-peer. LUX talks to owners and decision-makers as equals, not as
  a vendor trying to win business.

WHAT NEVER APPEARS IN A LUX SALES EMAIL
- "Just following up" or "Checking in" as subject lines or openers
- "I wanted to reach out" — just reach out
- "We would love the opportunity to..." — too eager
- "Please let me know if you have any questions" as a closing line
- Exclamation points
- Any claim that cannot be backed by a specific fact or example
- Generic compliments on the lead's project ("sounds like an exciting project!")

PERSONALIZATION RULES
- Always pull at least one specific detail from the lead's own message
  (their words, their project description, their timeline concern)
- Mirror their language — if they said "tenant spaces", use "tenant spaces",
  not "commercial units"
- The lead should feel like this email was written for them specifically,
  not generated from a template

SIGN-OFF
- Email 1: "Looking forward to it, / The LUX Team"
- Email 2 and 3: "— LUX"
```

---

### Prompt 2 — Lead Classification (user message, lightweight pre-call)

Run this as a small first API call before generating the sequence. It classifies the lead and extracts the single most useful detail from their message — feeding structured signal into Prompts 3A/3B rather than passing raw freeform text.

```
Analyze this inbound construction lead and return a classification.

Extract:
1. lead_tier: "high", "medium", or "low"
   - high: clear budget, near-term timeline, specific project description
   - medium: budget or timeline is vague, project is described generally
   - low: no budget signal, no timeline, very short or generic message
2. project_category: "custom_home", "commercial_renovation", "addition_or_remodel",
   "multi_family", or "unknown"
3. key_detail: the single most specific or useful thing the lead said in their
   own words — a phrase, a constraint, a number, a concern. Max 15 words.
   If nothing specific was said, return null.
4. urgency_signal: true if the lead mentioned a specific start date, deadline,
   or time pressure. Otherwise false.

Return a JSON object with exactly these four keys: "lead_tier", "project_category",
"key_detail", "urgency_signal".
Return JSON only. No preamble, no markdown fences.

Lead data:
{lead_json}
```

This output is passed as additional context into the sequence generation prompt, not shown to the user.

---

### Prompt 3A — Sequence Generator: Commercial Lead (user message)

Used when `project_category` is `commercial_renovation` or `multi_family`. Sent alongside Prompt 1 as `system`.

```
Write a 3-email follow-up sequence for a commercial construction lead.

CONTEXT FROM LEAD CLASSIFICATION
Lead tier: {lead_tier}
Key detail from their message: {key_detail}
Urgency signal: {urgency_signal}

EMAIL 1 — Send same day
Purpose: acknowledge, demonstrate immediate expertise, set up a call
- Open with the key_detail from their message — make it clear you actually read it
- In 1–2 sentences, show you understand the specific challenge of their
  project type (commercial work in occupied buildings, phased permitting,
  tenant coordination, business continuity during construction)
- One clear CTA: propose a specific call duration (20 minutes) and ask
  for their availability
- If urgency_signal is true: acknowledge the timeline explicitly
- Max 120 words

EMAIL 2 — Send day 3, only if no reply to Email 1
Purpose: add genuine value, re-engage without pressure
- Lead with one specific insight, consideration, or question relevant to
  their project category — something they may not have thought about
  (permitting timelines, phased construction approach, occupancy requirements,
  landlord approval processes for tenant renovations)
- This is not a pitch — it is useful information that positions LUX as
  someone worth talking to
- Soft CTA: one question or a low-friction meeting offer
- Max 120 words

EMAIL 3 — Send day 7, only if no reply to Email 1 or 2
Purpose: leave the door open, no pressure close
- Acknowledge that timing may have shifted
- One sentence on why LUX is selective about commercial projects (signals
  that this is not a desperate follow-up)
- Clear offer to reconnect whenever they are ready
- No CTA pressure — just an open door
- Max 100 words

SUBJECT LINE RULES FOR ALL THREE EMAILS
- Must reference something specific from their project or message
- Never use: "Following up", "Checking in", "Quick question", "Re:", or
  any variation of these
- Email 2 subject should reference the insight or consideration being shared
- Email 3 subject should feel like a natural close, not a last-ditch attempt

Return a JSON object with keys "email_1", "email_2", "email_3".
Each key contains "subject" and "body".
Body values use \n for line breaks.
Return JSON only. No preamble, no markdown fences.

Lead data:
{lead_json}
```

---

### Prompt 3B — Sequence Generator: Residential Lead (user message)

Used when `project_category` is `custom_home` or `addition_or_remodel`. Sent alongside Prompt 1 as `system`.

```
Write a 3-email follow-up sequence for a residential construction lead.

CONTEXT FROM LEAD CLASSIFICATION
Lead tier: {lead_tier}
Key detail from their message: {key_detail}
Urgency signal: {urgency_signal}

EMAIL 1 — Send same day
Purpose: acknowledge, build immediate personal connection, set up a call
- Open with the key_detail — show you read their specific situation
- Residential clients are making one of the largest financial decisions of
  their lives. The tone is warmer than commercial but still confident.
- 1–2 sentences that show you understand what they are trying to build —
  not the construction, the outcome (the home they want to live in)
- One clear CTA: propose a 20-minute call, ask for their availability
- If urgency_signal is true: acknowledge their timeline
- Max 120 words

EMAIL 2 — Send day 3, only if no reply to Email 1
Purpose: add value specific to residential builds, re-engage gently
- Lead with one consideration that is specific to their project type:
  custom home (design-build vs. design-bid-build, how early to engage a
  builder, what to have ready before the first meeting), addition or remodel
  (permit timelines, living in the home during construction, how scope creep
  happens and how LUX prevents it)
- Frame it as something most homeowners don't know until it's too late
- Soft CTA: offer a no-obligation conversation
- Max 120 words

EMAIL 3 — Send day 7, only if no reply to Email 1 or 2
Purpose: soft close, leave the door open
- Keep it brief and human — 3–4 sentences
- Acknowledge that projects like theirs take time to move forward
- Make it easy to come back without feeling like they owe LUX anything
- No pressure, no urgency manufacturing
- Max 100 words

SUBJECT LINE RULES FOR ALL THREE EMAILS
- Specific to their project or message — never generic
- Email 1: reference their project type or what they said they want to build
- Email 2: reference the insight or consideration being shared
- Email 3: feel like a genuine open door, not a last attempt

Return a JSON object with keys "email_1", "email_2", "email_3".
Each key contains "subject" and "body".
Body values use \n for line breaks.
Return JSON only. No preamble, no markdown fences.

Lead data:
{lead_json}
```

---

### Prompt 4 — Slack Notification Copy (user message)

Short third API call. Produces the internal Slack message the sales team sees.

```
Write a brief internal Slack message notifying the LUX sales team that a
new lead has come in and their follow-up sequence is staged in Airtable.

Rules:
- 3–4 sentences max
- Casual internal tone — this is a team channel
- Include: lead name, company (if commercial), project type, budget range,
  timeline, lead tier, and whether there is a urgency signal
- If lead_tier is "high": flag it clearly — this one needs a same-day
  human touch on top of the automated sequence
- If lead_tier is "low": note it briefly — sequence will run but may not
  be worth heavy manual follow-up
- End with a direct question: who is owning this lead?
- Do not summarize the emails

Return a JSON object with one key: "slack_message".
Return JSON only. No preamble, no markdown fences.

Context:
{lead_context_json}
```

Where `lead_context_json` is assembled by a Code node after Prompt 2 runs:

```json
{
  "name": "Marcus Webb",
  "company": "Webb Commercial Properties",
  "project_type": "commercial_renovation",
  "budget_range": "250k-500k",
  "timeline": "Q3 2025",
  "lead_tier": "high",
  "urgency_signal": false,
  "key_detail": "two tenant spaces in our office building downtown"
}
```

---

### Prompt Routing Logic in n8n

```
Webhook
  → HTTP Request (Prompt 1 + 2: lead classification)
  → JSON Parse (extract lead_tier, project_category, key_detail, urgency_signal)
  → Code node (build lead_context_json, inject classification into sequence prompt)
  → IF node: project_category in ["commercial_renovation", "multi_family"]
      → True:  HTTP Request (Prompt 1 + 3A: commercial sequence)
      → False: HTTP Request (Prompt 1 + 3B: residential sequence)
  → Merge
  → JSON Parse (extract email_1, email_2, email_3)
  → Airtable (create record with all fields)
  → HTTP Request (Prompt 4: Slack notification copy)
  → JSON Parse (extract slack_message)
  → Slack (post to #new-leads)
```

The Code node between classification and sequence generation:

```javascript
// n8n Code node — build enriched context for sequence prompt and Slack
const classification = $input.first().json;
const lead = $('Webhook').first().json;

return {
  lead_tier: classification.lead_tier,
  project_category: classification.project_category,
  key_detail: classification.key_detail,
  urgency_signal: classification.urgency_signal,
  lead_context_json: JSON.stringify({
    name: lead.name,
    company: lead.company || null,
    project_type: lead.project_type,
    budget_range: lead.budget_range,
    timeline: lead.timeline,
    lead_tier: classification.lead_tier,
    urgency_signal: classification.urgency_signal,
    key_detail: classification.key_detail
  })
};
```

This is the only code in the pipeline.

---

## Output

Claude returns:

```json
{
  "email_1": {
    "subject": "Your tenant space renovations — let's talk scope",
    "body": "Hi Marcus,\n\nThanks for reaching out. Two tenant spaces in a live office building is exactly the kind of project we do well — phased work, minimal disruption to existing tenants, tight coordination.\n\nI'd like to set up a 20-minute call to understand your timeline and what you're working with. Are you available this week or early next?\n\nLooking forward to it,\nThe LUX Team"
  },
  "email_2": {
    "subject": "One thing worth knowing before scoping your renovation",
    "body": "Hi Marcus,\n\nOne thing that often catches commercial clients off guard: tenant space renovations in occupied buildings almost always require phased permitting, which can add 2–3 weeks to a Q3 start if not accounted for early.\n\nHappy to walk you through how we've handled this on similar projects — it's solvable, just worth planning for.\n\nStill a good time to connect this week?\n\n— LUX"
  },
  "email_3": {
    "subject": "Hartwell Residence — still happy to help when the time is right",
    "body": "Hi Marcus,\n\nJust wanted to leave this here in case the timing shifted. We're selective about the commercial projects we take on, and yours sounds like a good fit.\n\nWhenever you're ready to move forward — or just want a second opinion on scope — we're here.\n\n— LUX"
  }
}
```

---

## Airtable Schema

Table: `Leads`

| Field | Type | Notes |
|---|---|---|
| Name | Text | |
| Company | Text | |
| Project Type | Single select | custom_home, commercial_renovation, addition_or_remodel, multi_family |
| Source | Text | website, referral, etc. |
| Budget Range | Text | |
| Timeline | Text | |
| Original Message | Long text | |
| Lead Tier | Single select | high / medium / low — from Prompt 2 classification |
| Key Detail | Text | Extracted phrase from lead's message |
| Urgency Signal | Checkbox | true if lead mentioned a deadline or start date |
| Email 1 Subject | Text | |
| Email 1 Body | Long text | |
| Email 2 Subject | Text | |
| Email 2 Body | Long text | |
| Email 3 Subject | Text | |
| Email 3 Body | Long text | |
| Status | Single select | New / Contacted / Qualified / Dead |
| Created | Date | Auto |

---

## Delivery

Slack message to `#new-leads`:

```
🔔 *New Lead — Sequence Ready*
*Name:* Marcus Webb
*Company:* Webb Commercial Properties
*Project:* Commercial Renovation
*Budget:* $250k–$500k
*Timeline:* Q3 2025

3 follow-up emails drafted and staged in Airtable.
👉 Review: https://airtable.com/...
```

---

## Build Path

**Tool:** n8n

| Step | Node | Config |
|---|---|---|
| 1 | Webhook | POST trigger, accepts lead JSON |
| 2 | HTTP Request | Anthropic API — Prompt 1 (system) + Prompt 2 (lead classification) |
| 3 | JSON Parse | Extract `lead_tier`, `project_category`, `key_detail`, `urgency_signal` |
| 4 | Code | Build enriched context object, inject classification into sequence prompt |
| 5 | IF | Route on `project_category`: commercial → 6A, residential → 6B |
| 6A | HTTP Request | Anthropic API — Prompt 1 (system) + Prompt 3A (commercial sequence) |
| 6B | HTTP Request | Anthropic API — Prompt 1 (system) + Prompt 3B (residential sequence) |
| 7 | Merge | Rejoin both branches |
| 8 | JSON Parse | Extract `email_1`, `email_2`, `email_3` with subjects and bodies |
| 9 | Airtable | Create record, map all 8 fields + classification fields |
| 10 | HTTP Request | Anthropic API — Prompt 4 (Slack notification copy) |
| 11 | JSON Parse | Extract `slack_message` |
| 12 | Slack | Post to `#new-leads` with Airtable deep link |

**Estimated build time:** 5–6 hours

---

## Demo Script (Loom)

1. Show both payloads side by side — commercial (Marcus Webb) and residential (alt payload)
2. Trigger the commercial payload, walk through each n8n node:
    - Show the classification output from Prompt 2 — point out `lead_tier: "high"` and `key_detail`
    - Show the IF node routing to the commercial sequence prompt
    - Show the Airtable record populate — point out the classification fields alongside the email drafts
    - Show the Slack notification land — point out it flags this as a high-tier lead and asks who is owning it
3. Open Email 1 in Airtable — show Marcus's exact words ("two tenant spaces") appear in the first sentence
4. Open Email 2 — show the commercial-specific insight (phased permitting) that a residential prompt would never produce
5. Trigger the residential payload — show the IF node routes to 3B, show Email 2 is completely different (design-build consideration vs. permitting)
6. Point out the subject lines — none say "Following up" or "Checking in"

**Key talking points:**
- "It classifies the lead before it writes — so the sequence is calibrated to the lead's quality and category, not just their name."
- "The `key_detail` extraction means Claude mirrors the lead's own language. Marcus said 'tenant spaces' — that's what shows up in Email 1."
- "Two prompts, two completely different sequences. The routing is built into the workflow, not hardcoded into one giant prompt."

---

## Input

**payload.json** — commercial lead, high tier, specific message:

```json
{
  "name": "Marcus Webb",
  "company": "Webb Commercial Properties",
  "source": "website_contact_form",
  "project_type": "commercial_renovation",
  "budget_range": "250k-500k",
  "timeline": "Q3 2025",
  "message": "Looking to renovate two tenant spaces in our office building downtown. Want to discuss scope and timeline."
}
```

**payload_alt.json** — residential lead, medium tier, vague message:

```json
{
  "name": "Jennifer Okafor",
  "company": null,
  "source": "referral",
  "project_type": "custom_home",
  "budget_range": "800k-1.2m",
  "timeline": "Not sure yet, maybe next year",
  "message": "We were referred by the Garcias. Looking to build a custom home on a lot we own in the area. Early stages, just starting to talk to builders."
}
```

The alt payload exercises the residential prompt path, a medium lead tier, and a vague timeline — the Email 3 soft close will land very differently here than it does for Marcus.

---

## Files

```
mvp-b/
├── README.md                      ← this file
├── payload.json                   ← sample lead payload (commercial, high tier)
├── payload_alt.json               ← sample lead payload (residential, medium tier)
├── prompts/
│   ├── system_brand_voice.txt     ← Prompt 1, applied to every API call
│   ├── user_classify.txt          ← Prompt 2, lead classification
│   ├── user_sequence_commercial.txt ← Prompt 3A, commercial sequence
│   ├── user_sequence_residential.txt ← Prompt 3B, residential sequence
│   └── user_slack_notify.txt      ← Prompt 4, internal Slack notification copy
└── n8n_workflow.json              ← exportable n8n workflow
```

---

## Bruno Endpoint Index

Collection: `GroupScout API` → `LUX MVP-B`
Environment variable: `{{n8n_url}}` (default: `http://localhost:5678`)

| # | File | Method | URL | Payload |
|---|---|---|---|---|
| 1 | `Lead Follow-Up - Commercial.bru` | POST | `{{n8n_url}}/webhook/lux-lead-followup` | Marcus Webb — commercial renovation, high tier |
| 2 | `Lead Follow-Up - Residential.bru` | POST | `{{n8n_url}}/webhook/lux-lead-followup` | Jennifer Okafor — custom home, medium tier |

Both requests hit the same webhook URL. The n8n workflow routes internally based on `project_type` from the payload.

**Expected results per request:**
- Commercial payload → IF node routes to Prompt 3A → Airtable record with phased permitting insight in Email 2
- Residential payload → IF node routes to Prompt 3B → Airtable record with design-build consideration in Email 2
- Both → Slack notification posted to `#new-leads` with lead tier flagged