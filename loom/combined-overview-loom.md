# Combined Loom Draft

## Goal

Walk through all three MVPs in one 7-minute Loom: a short introduction, MVP A, MVP B, MVP C, then a wrap-up.

## Audience

Operators, founders, and stakeholders who want one clear overview of what the MVP set does.

## Recommended Screen Flow

Start with a simple repo overview or README heading:

```text
LUX Automation MVP Suite
MVP A: Client Status Updates
MVP B: Lead Follow-Up
MVP C: Content Pipeline
```

Then move through:

1. `docs/mvps/`
2. `docs/mvps/mvp-a/README.md`
3. `docs/mvps/mvp-b/README.md`
4. `docs/mvps/mvp-c.md`
5. `loom/mvp-a-loom.md`, `loom/mvp-b-loom.md`, `loom/mvp-c-loom.md` as speaker notes if needed

## Timed Talk Track

### 0:00-0:35 Introduction

"Hi, my name is Alvin. I put together three MVPs for LUX to show how I would approach this role from day one."

"The pattern across all three is simple: take structured business input, use AI to create a useful first draft, and keep a human in control before anything goes to a client, a lead, or the public."

"I chose these three because they map directly to the role: client communication, sales follow-up, and marketing content systems. MVP A handles client status updates, MVP B handles lead follow-up, and MVP C handles LinkedIn and podcast content."

### 0:35-2:20 MVP A

"MVP A handles client status updates. I built it to take structured project data, draft a client-ready email, and drop that draft into Slack for PM review, so nothing goes to the client unless somebody approves it."

"The reason I picked this use case is pretty simple: PMs spend a lot of time pulling details from JobTread, memory, and site notes, then writing the same kind of update again and again, so this turns that into a quick first draft and keeps the tone consistent."

"One choice I made here was to keep the input structured. It includes things like phase, percent complete, schedule detail, milestones, budget context, and open items, which matters because Claude is not being asked to infer everything from a messy paragraph. It’s working from a clean snapshot of the project."

"The workflow itself is pretty straightforward: the webhook receives the payload, a Code node builds context, another node loads prompts from the repo, Claude writes the email, and then a lighter Claude call writes the Slack note. By the end of that flow, Slack has a review-ready draft with approve or edit options."

"The main design choice here was control, so I treated the client email and the internal Slack note as two different jobs. I also made the prompt rules explicit, so budget issues and client action items get handled carefully instead of improvised."

### 2:20-4:10 MVP B

"MVP B handles lead follow-up. I built it to take an inbound lead, classify it, generate a three-email sequence, store the drafts in Airtable, and ping the sales team in Slack."

"The big idea here is that it classifies before it writes. A lot of AI follow-up demos jump straight from form submission to email copy, but I wanted a reasoning step first so the writing actually matches the lead quality and the project category."

"That matters because a commercial renovation lead and a custom home lead shouldn’t get the same follow-up. In this workflow, commercial categories route down one branch and residential categories go down another."

"Once the lead is classified, the workflow builds context, generates the right sequence, writes everything into Airtable, and sends a Slack notification so the sales team knows who should take ownership."

"The real value is that the draft feels tailored to the lead instead of sounding like a template, because it uses the lead’s own language, picks the right angle, and gives the team a review layer before anything gets sent."

### 4:10-5:55 MVP C

"MVP C handles content generation. I built it to take either a project milestone or a podcast episode, turn that into a LinkedIn draft in LUX’s voice, and send the result to Slack for review."

"This solves a different problem, but the pattern is the same, because LUX already has valuable content in the business. Project wins, milestones, and podcast episodes all contain material that could become social posts, and the bottleneck is usually turning that material into consistent copy."

"This workflow supports two content types: if the payload is a project milestone, it routes to one prompt, and if it’s a podcast episode, it routes to another. Both share the same brand voice rules, but the writing logic changes based on the content."

"That branching is important, because a milestone post and a contractor podcast post shouldn’t sound the same. The milestone post leads with proof and progress. The podcast post leads with tension or a sharper business insight."

"And again, the workflow stops at review rather than auto-posting, so the team gets the draft in Slack, makes edits if needed, and decides whether it should go live."

### 5:55-7:00 Wrap-Up

"Across all three MVPs, the pattern is the same: structured input comes in, AI creates the first draft, and the team reviews before anything important goes out."

"That is the main thing I wanted to show. I am not trying to automate judgment away. I am trying to remove the repetitive drafting work around the judgment, so the team can move faster without losing control."

"The same pattern can extend into schedule alerts, budget variance reporting, subcontractor onboarding, internal briefings, and agent platform workflows."

"If I had to summarize the approach in one line, it would be this: automate the first draft, not the final decision."

"Thanks for watching. I would be happy to walk through the workflows, prompt structure, or n8n setup in more detail."

## Key Points To Land

- All three MVPs start with structured input, not freeform guessing.
- Each workflow uses AI for draft generation, not blind automation.
- Slack and Airtable act as review layers, not just delivery tools.
- The common pattern is speed plus control.

## Close

"There’s obviously more we could get into on each of these, but for this overview, what I’d want you to take away is how I think about workflow design, AI guardrails, and human review, and how that lines up with the kind of systems you’re looking to build at LUX. Thanks for taking the time to watch, and I hope to hear from you soon."
