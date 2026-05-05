# Combined Loom Draft

## Goal

Walk through all three MVPs in one 7-minute Loom: a short introduction, MVP A, MVP B, MVP C, then a wrap-up.

## Audience

Operators, founders, and stakeholders who want one clear overview of what the MVP set does.

## Recommended Screen Flow

1. `docs/mvps/`
2. `docs/mvps/mvp-a/README.md`
3. `docs/mvps/mvp-b/README.md`
4. `docs/mvps/mvp-c.md`
5. `loom/mvp-a-loom.md`, `loom/mvp-b-loom.md`, `loom/mvp-c-loom.md` as speaker notes if needed

## Timed Talk Track

### 0:00-0:35 Introduction

"Hi, my name is Alvin. Thanks for taking the time to watch this. I put together three MVPs for LUX, and they all follow the same basic idea: take structured business input, use AI to create a solid first draft, and keep a human in the loop before anything goes out."

"I picked these three on purpose because they map pretty closely to the kind of work in the role, especially around client communication, sales workflow automation, and marketing content systems. More specifically, MVP A is client status updates, MVP B is lead follow-up, and MVP C is LinkedIn and podcast content. I’ll move through each one pretty quickly, talk through a few of the choices I made, and then wrap up with the pattern that ties all three together."

### 0:35-2:20 MVP A

"MVP A handles client status updates. I built it to take structured project data, draft a client-ready email, and drop that draft into Slack for PM review, so nothing goes to the client unless somebody approves it."

"The reason I picked this use case is pretty simple: PMs spend a lot of time pulling details from JobTread, memory, and site notes, then writing the same kind of update again and again, so this turns that into a quick first draft and keeps the tone consistent."

"One choice I made here was to keep the input structured. It includes things like phase, percent complete, schedule detail, milestones, budget context, and open items, which matters because Claude isn’t guessing from a messy paragraph. It’s working from a clean snapshot of the project."

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

"That branching is important, because a milestone post and a contractor podcast post shouldn’t sound the same. One leads with proof and progress, while the other leads with tension or a sharper insight."

"And again, the workflow stops at review rather than auto-posting, so the team gets the draft in Slack, makes edits if needed, and decides whether it should go live."

### 5:55-7:00 Wrap-Up

"Across all three MVPs, the pattern stays consistent: structured input comes in, AI produces a useful first draft, and a human reviews the output before it goes anywhere important."

"That’s really what I wanted to show with this MVP set. It’s not AI for the sake of AI, but AI applied to specific business workflows where speed matters, consistency matters, and human judgment still matters. Even though these examples focus on communication, sales, and content, the same approach carries over pretty naturally into operations, reporting, internal workflows, and agent platform work."

"If I had to sum up my approach in one line, it would be this: automate the first draft, not the final decision."

## Key Points To Land

- All three MVPs start with structured input, not freeform guessing.
- Each workflow uses AI for draft generation, not blind automation.
- Slack and Airtable act as review layers, not just delivery tools.
- The common pattern is speed plus control.

## Close

"There’s obviously more we could get into on each of these, but for this overview, what I’d want you to take away is how I think about workflow design, AI guardrails, and human review, and how that lines up with the kind of systems you’re looking to build at LUX. If it’d be helpful, I’d be glad to talk through any of this in more detail on a call. Thanks for taking the time to watch, and I hope to hear from you soon."
