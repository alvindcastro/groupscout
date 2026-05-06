# LinkedIn Post Drafts For GroupScout

Audience: hiring managers, CTOs, and technical leaders.

Tone: casual, specific, builder-focused. AI should read as an assistive layer, not the whole story.

Flow cues from `docs/eos.md`:

- Use active voice.
- Prefer concrete details over broad claims.
- Keep each post focused on one subject.
- Use full paragraphs instead of stacking too many single-sentence lines.
- Avoid em dashes.
- End with the strongest engineering or product point.

## Draft 1 - Finding Demand Before It Is Obvious

I've been playing with a Go project called GroupScout, a lead intelligence tool for hotel sales teams. The problem is practical: construction crews, film crews, conference groups, and disrupted passengers all create lodging demand, but hotels often spot the signal after someone else has already won the business.

The raw signals are public. Building permits, bid notices, film production lists, convention calendars, infrastructure announcements, and airport disruption feeds all say something useful if you collect them early enough. GroupScout pulls those sources into one pipeline, deduplicates the records, filters the noise, and turns the useful items into Slack and email alerts.

AI helps in the middle of the workflow. It estimates things like crew size, duration, room-night potential, priority, and outreach timing. The software still owns the collection, storage, rules, and delivery.

That split is what makes the project interesting to me. It is not an AI wrapper looking for a job. It is a workflow with a real user, a real business outcome, and AI used where interpretation helps.

## Draft 2 - The Parser Bug That Made The System Better

One of the better lessons from GroupScout came from a boring parser bug. Richmond publishes building permits as PDFs, and I wanted to pull out commercial projects, estimate lodging demand, and send the best leads to Slack.

The first version worked against clean fixtures, then failed on the real PDF output. Every permit showed a construction value of `$1`, which made the pipeline look like it had a threshold or config problem. After dumping the raw `pdftotext` output, the issue was obvious: the PDF included a bare `1` between the issue date and the dollar value, and that value was a permit count row.

The original parser trusted position. Line 3 was supposed to be the construction value, so it happily stored `$1` and moved the real value into the wrong field. I rewrote the parser to identify fields by content instead: dates look like dates, dollar values look like dollar values, address rows start with `FOLDER NAME`, and bare count rows can be skipped.

That change made the collector more robust than the clean test version. Real data has texture, and the system got better once the code listened to the shape of the source instead of the shape of the fixture.

## Draft 3 - Keeping AI In Its Lane

The AI part of GroupScout is intentionally narrow. The app watches public signals for hotel group-sales opportunities, but most of the system is plain engineering: scrape the source, parse the rows, store the raw record, hash it for deduplication, and reject low-value items before they cost an API call.

Only then does the model enter the pipeline. It reads the context and estimates what a hotel sales manager would care about, such as whether the project is likely to bring out-of-town crews, how long they might stay, and whether the lead deserves an urgent call or a weekly digest.

I like that boundary because it keeps the workflow inspectable. Go code owns state, retries, filtering, storage, and notifications. The model adds interpretation where rigid rules get clumsy.

That is the AI pattern I keep coming back to. Use deterministic software for the parts that must be reliable, then use the model as an assistant for judgment.

## Draft 4 - Product Thinking In A Side Project

GroupScout started as a side project, but I tried to treat it like a product. The intended user is a hotel sales manager, and the job is not "show me more data." The job is "tell me which lead deserves attention before a competitor calls them."

That framing shaped the architecture. Raw public records go into storage first, so a failed enrichment step does not lose the source data. Each record gets a dedup hash, so repeated permit reports do not burn duplicate AI calls. A pre-scorer rejects low-value residential jobs before they reach the model.

The output goes where the user already works. Urgent leads go to Slack, weekly review goes to email, and n8n can trigger the API on a schedule. The airport disruption monitor runs as its own `alertd` binary because it has a different cadence and failure mode from the lead pipeline.

That is the part I would want a hiring manager or CTO to notice. The repo is not just a prompt connected to an API. It is a set of product decisions expressed in code.

## Draft 5 - Why Go Fits This Project

I built GroupScout in Go because the project is mostly background pipeline work. It collects data from public sources, normalizes each item, stores it, enriches it, and sends notifications. That shape does not need a heavy framework.

The core pieces stay small. Collectors implement a common interface, storage handles raw records and enriched leads, enrichment wraps the active LLM provider, and notification sends Slack cards or email digests. Adding a new source means returning the same `RawProject` shape, not rewriting the pipeline.

Go also makes the operational parts feel straightforward. The server can expose trigger endpoints, the CLI can run the pipeline once, and `alertd` can run as a separate long-lived process. The repo can support local SQLite, production Postgres, Docker Compose, and n8n without turning into framework archaeology.

The fun part is pairing boring reliability with AI assistance. Go keeps the system grounded, and the model helps interpret messy context when rules alone get awkward.

## Draft 6 - Cost Control Before AI Calls

One design choice I care about in GroupScout is that the pipeline tries not to call AI unless the lead is worth it. Public sources include a lot of noise: small repairs, residential renovations, repeated permit rows, old bid notices, and events that have no real group lodging potential.

So the pipeline does cheap work first. It stores raw records, calculates a SHA-256 dedup hash, skips anything already seen, and runs a rule-based pre-scorer before enrichment. That pre-scorer can reject low-value work by project type, budget, source, and keywords.

The model only sees the higher-potential items. At that point, AI is doing a judgment task instead of acting like an expensive filter for data the code could have rejected. It estimates lodging potential, explains the score, and drafts outreach context.

This is a small thing, but it matters in production-minded AI work. The fastest, cheapest, most reliable model call is the one you never make.

## Draft 7 - Slack As The Product Surface

A lot of GroupScout's value lives in the final mile. The pipeline can collect permits, bids, events, and film productions all day, but the user still needs a clear answer: should I act on this lead?

That is why Slack became the main product surface. High-priority leads turn into compact cards with the project type, address, budget, likely contractor, score, outreach timing, and source link. The goal is not to make the sales team inspect raw data. The goal is to help them decide whether to claim, dismiss, snooze, or follow up.

AI helps write the summary and rationale, but the alert still needs product discipline. A vague score is not enough. The message has to explain why the lead matters, what to do next, and where the source came from.

That has been a useful reminder while building with AI. The model output is only valuable if it lands inside a workflow where someone can act on it.

## Draft 8 - Airport Disruptions As A Separate System

GroupScout has a lead pipeline, but it also has a separate airport disruption monitor called `alertd`. I split it out because airport disruption alerts have different timing, urgency, and failure modes from weekly lead collection.

The lead pipeline can run on a schedule. It can collect permits, bids, events, and news, then send a digest or urgent Slack alert. Airport disruption monitoring is different. It needs to poll weather, YVR cancellation data, and NOTAMs every few minutes, then decide whether conditions are likely to create same-day hotel demand.

`alertd` computes a Stranded Passenger Score from cancellation rate, aircraft seats, passenger mix, time of day, and disruption duration. It uses a lifecycle model so the system can watch, alert, update, and resolve without spamming a channel.

This was a good architecture exercise. Sometimes the right move is not to make one pipeline do everything. Sometimes the right move is to admit that two workflows have different physics and give them separate runtimes.

## Draft 9 - Local First, Production Ready

I wanted GroupScout to be easy to run locally without giving up a path to production. That led to a storage layer that supports SQLite for local development and Postgres for deployment.

SQLite keeps the feedback loop simple. I can run the pipeline, inspect records, and test collectors without setting up infrastructure. Postgres adds the production path, including migrations, `pgx`, and pgvector support for similarity search.

The interesting part is keeping the application code from caring too much about the driver. Storage handles the differences, including placeholder rebinding and schema setup. The rest of the pipeline can focus on raw projects, enriched leads, outreach logs, and embeddings.

That kind of portability is not flashy, but it makes side projects easier to finish and real systems easier to operate. I like when the boring pieces reduce friction instead of adding ceremony.

## Draft 10 - What I Would Want A CTO To Notice

If a CTO or hiring manager skimmed GroupScout, I would want them to notice the boundaries more than the buzzwords. The project uses AI, but the architecture does not ask the model to own the system.

The repo has collectors for public data, a storage layer with deduplication and auditability, rule-based pre-scoring, LLM enrichment, Slack and email delivery, n8n-friendly endpoints, Docker deployment, observability hooks, and a separate airport disruption daemon. The model helps interpret messy business context, but the code still controls state and flow.

That is the kind of AI engineering I find most useful right now. It asks normal software questions first: What is the workflow? What can fail? What should be stored? What should be skipped? What does the user need to decide?

Once those answers are clear, AI becomes a sharp assistive layer instead of a vague center of gravity.

## CTA Options

- Curious how other teams are drawing the line between deterministic code and AI judgment.
- Would be interested to hear how other teams use public data as an early signal.
- This has been a useful playground for practical AI systems: less demo magic, more workflow leverage.
- For teams building with AI in production, where are you keeping the model out of the critical path?
- What is your favorite example of AI helping a workflow without owning the workflow?
