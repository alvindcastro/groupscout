# MVP C Loom Draft

## Goal

Show how LUX turns a project milestone or a podcast episode into a branded LinkedIn draft, then sends it to Slack for review.

## Audience

Brand, content, and operations stakeholders who want a repeatable content pipeline.

## Recommended Screen Flow

1. `docs/mvps/mvp-c.md`
2. `docs/mvps/mvp-c/payload_milestone.json`
3. `docs/mvps/mvp-c/payload_podcast.json`
4. `docs/mvps/mvp-c/n8n_workflow.json` in n8n
5. Slack output in `#content-review`

## Timed Talk Track

### 0:00-0:30

"This MVP writes LinkedIn posts for LUX. It supports two content types: project milestones and Built Different podcast episodes. The workflow takes structured input, writes a post in LUX’s voice, and sends the draft to Slack for review."

### 0:30-1:00

"The problem is simple. LUX has valuable content, but turning project progress or podcast insight into clean social posts takes time and usually depends on who writes it that day. This pipeline makes the output fast and consistent."

### 1:00-1:45

"Here’s the first payload, a project milestone. It includes the milestone, a concrete detail, the team involved, and the next phase. The prompt is tuned to lead with facts like size, duration, or a challenge overcome, not generic milestone language."

"The second payload is a podcast episode. It includes the guest, topic, and key takeaways. The prompt logic is different here. Instead of recapping the episode, it surfaces the sharpest or most counterintuitive idea so the post actually earns attention."

### 1:45-2:35

"In n8n, the webhook receives the payload and an IF node routes by `type`. Project milestones go to one Claude call. Podcast episodes go to another. Both branches share the same brand voice system prompt, then merge into one path where the post is extracted, Slack copy is generated, and the final message goes to `#content-review`."

"That branching matters because the two content types should not use one generic social prompt. A milestone post and a contractor podcast post need different hooks, different audience assumptions, and different calls to action."

### 2:35-3:20

"The brand voice rules do a lot of the heavy lifting. They ban soft corporate phrasing, force a strong first line, cap the total length, and require tight formatting. That keeps the output on brand and avoids the usual AI mush."

"For milestone posts, the workflow emphasizes proof: square footage, time saved, obstacles handled. For podcast posts, it emphasizes tension: the idea that makes another contractor stop scrolling."

### 3:20-4:05

"The last step is Slack delivery. Instead of auto-posting to LinkedIn or Buffer, the workflow sends the draft into a human review lane. The team can edit the tone, add context, or decide whether to schedule it at all."

"That’s the right constraint for content automation. Brand content needs speed, but it also needs judgment. Slack becomes the checkpoint before anything goes public."

### 4:05-4:40

"To close the demo, I’d show both outputs back to back. One should read like a sharp construction update. The other should read like peer-to-peer content for contractors. Same company voice, different content strategy. That’s the proof that the routing and prompt design work."

## Key Points To Land

- One webhook supports two distinct content workflows.
- Shared brand voice plus separate user prompts keeps output consistent but not repetitive.
- The workflow optimizes for reviewable drafts, not auto-publishing.
- Structured inputs make it easy to scale content creation without flattening the brand voice.

## Close

"MVP C turns internal business activity into publishable content with very little manual effort. The first win is speed and consistency. The bigger win is that the team can stay active on LinkedIn without making content a separate job."
