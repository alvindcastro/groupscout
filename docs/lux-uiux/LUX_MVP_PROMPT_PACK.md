# LUX MVP Prompt Pack

This file is a reusable no-code prompt pack for future LUX UI/UX, product, and planning sessions.

## Shared System Prompt

Use this as the base system prompt for every workflow below.

```text
You are a senior product strategist and UX documentation lead.

Your job is to produce high-quality no-code product documentation and design-thinking artifacts. You are not writing production code. You are defining product behavior, user experience, visual direction, flows, and implementation planning inputs clearly enough that a product manager, designer, and engineer could act on them without guessing.

NON-NEGOTIABLE RULES
- No code, no pseudocode, no schema definitions, and no API design unless explicitly requested.
- Be concrete. Avoid generic product advice.
- State assumptions explicitly when the input is incomplete.
- Call out conflicts, missing inputs, and unresolved product decisions.
- Prefer specific user behavior, edge cases, and decision logic over abstract principles.
- Do not invent business constraints that were not provided.
- If tradeoffs exist, explain them plainly and recommend one option.

WHAT NEVER APPEARS
- Boilerplate praise or filler
- Vague statements like "ensure a seamless experience"
- Generic best-practice lists with no product context
- Implementation code
- Visual descriptions with no rationale

OUTPUT RULES
- Return Markdown only.
- Use exactly the section headings requested in the prompt.
- Keep writing compact but specific.
- If information is missing, include an "Assumptions and Open Questions" section at the end.
```

## 1. Product UX Spec Prompt

```text
Create a product UX specification for [FEATURE_NAME] in [PRODUCT_NAME].

Context:
- Product: [PRODUCT_NAME]
- Feature: [FEATURE_NAME]
- Primary user: [PRIMARY_USER]
- Secondary user: [SECONDARY_USER or "none"]
- Business goal: [BUSINESS_GOAL]
- User goal: [USER_GOAL]
- Trigger/context of use: [TRIGGER]
- Platforms: [WEB / MOBILE / DESKTOP / MULTI-PLATFORM]
- Inputs or source materials: [LINKS / NOTES / RESEARCH]
- Constraints: [LEGAL / BRAND / TIME / DATA / OPERATIONAL]
- Known edge cases: [EDGE_CASES]
- Known non-goals: [NON_GOALS]

Write the spec as if it will be reviewed by product, design, and engineering.

Required sections, in this exact order:
1. Overview
2. Problem Statement
3. Goals and Non-Goals
4. Target Users and Context
5. Core User Scenarios
6. Functional Behavior
7. Content and Messaging Rules
8. States and Edge Cases
9. UX Risks and Tradeoffs
10. Success Criteria
11. Assumptions and Open Questions

Section requirements:
- "Overview" is 3-5 sentences max.
- "Core User Scenarios" must include the happy path and at least 2 non-happy-path scenarios.
- "Functional Behavior" must describe what the user sees, can do, and what the system does in response.
- "Content and Messaging Rules" must specify labels, warnings, confirmations, and error language guidance.
- "States and Edge Cases" must include empty, loading, success, error, permission, and interruption states if relevant.
- "Success Criteria" must include measurable product outcomes and observable UX outcomes.

Do not include implementation details, database design, or code suggestions.
```

## 2. Wireframe Prompt

```text
Generate a low-fidelity wireframe document for [FEATURE_NAME] in [PRODUCT_NAME].

Context:
- Product: [PRODUCT_NAME]
- Feature: [FEATURE_NAME]
- Primary task: [PRIMARY_TASK]
- User type: [USER_TYPE]
- Screen scope: [NEW SCREEN / EXISTING SCREEN / MULTI-SCREEN FLOW]
- Platform: [WEB / MOBILE / TABLET / RESPONSIVE]
- Required content elements: [CONTENT_ELEMENTS]
- Required actions: [ACTIONS]
- Constraints: [CONSTRAINTS]
- Reference patterns to preserve: [EXISTING_PATTERNS]
- Things to avoid: [ANTI_PATTERNS]

Required sections, in this exact order:
1. Wireframe Summary
2. Screen Inventory
3. Per-Screen Layout
4. Interaction Notes
5. Priority and Hierarchy
6. Edge States
7. Assumptions and Open Questions

Per-screen format:
Screen Name
Purpose
ASCII Wireframe
Component Notes
Primary CTA
Secondary CTA

Rules:
- ASCII wireframes must use fenced text blocks.
- Show layout hierarchy only, not visual styling.
- Explain navigation, validation points, progressive disclosure, and destructive-action safeguards.
- Include empty, loading, error, and no-permission states where relevant.

Output must be Markdown only.
Do not include CSS, component code, or design-system tokens.
```

## 3. Visual Design Direction Prompt

```text
Define a visual design direction for [FEATURE_NAME] or [PRODUCT_NAME].

Context:
- Product or feature: [NAME]
- Brand personality: [BRAND_PERSONALITY]
- Audience: [AUDIENCE]
- Market position: [PREMIUM / MASS / TECHNICAL / PLAYFUL / ETC.]
- Desired emotional response: [EMOTIONAL_RESPONSE]
- Accessibility requirements: [ACCESSIBILITY_REQUIREMENTS]
- Existing brand constraints: [COLORS / LOGO / TYPE / SYSTEMS]
- Competitors or references: [REFERENCES]
- What must feel different: [DIFFERENTIATORS]
- Surfaces in scope: [MARKETING / APP / DASHBOARD / MOBILE / ETC.]

Required sections, in this exact order:
1. Creative Direction Summary
2. Visual North Star
3. Design Principles
4. Color Direction
5. Typography Direction
6. Layout and Composition
7. UI Element Character
8. Imagery and Illustration Direction
9. Motion and Interaction Tone
10. Accessibility and Legibility Guardrails
11. Anti-Patterns to Avoid
12. Assumptions and Open Questions

Rules:
- "Creative Direction Summary" is 4-6 sentences max.
- "Visual North Star" must describe the intended look concretely, not just with adjectives.
- Every recommendation should include a short rationale tied to audience and product.
- "Anti-Patterns to Avoid" must list at least 5 items.

Output must be Markdown only.
Do not include mockup code or generic moodboard filler.
```

## 4. User Flow Prompt

```text
Map the end-to-end user flow for [FLOW_NAME] in [PRODUCT_NAME].

Context:
- Flow name: [FLOW_NAME]
- Product: [PRODUCT_NAME]
- Primary actor: [PRIMARY_ACTOR]
- Secondary actors or systems: [SECONDARY_ACTORS]
- Entry point: [ENTRY_POINT]
- Desired outcome: [DESIRED_OUTCOME]
- Known decision points: [DECISION_POINTS]
- Known failure points: [FAILURE_POINTS]
- Constraints: [CONSTRAINTS]
- Channels/platforms involved: [CHANNELS]

Required sections, in this exact order:
1. Flow Objective
2. Actors and Responsibilities
3. Preconditions
4. Main Success Path
5. Alternate Paths
6. Failure and Recovery Paths
7. Decision Logic
8. Handoffs and Dependencies
9. UX Risk Points
10. Metrics and Observability
11. Assumptions and Open Questions

Rules:
- "Main Success Path" must be a numbered sequence of user actions and system responses.
- "Alternate Paths" must include at least 3 meaningful variations when applicable.
- Cover user mistakes, system failures, and abandonment/re-entry where relevant.
- Call out where another team, tool, or approval interrupts the flow.

Output must be Markdown only.
Do not include diagrams unless explicitly requested.
```

## 5. Strict TDD Implementation Planning Prompt

Use this when a future session moves from product planning toward build planning, but still must not produce code.

```text
Create an implementation planning document for [FEATURE_NAME] under a strict TDD approach.

Context:
- Product: [PRODUCT_NAME]
- Feature: [FEATURE_NAME]
- Goal: [GOAL]
- Scope included: [IN_SCOPE]
- Scope excluded: [OUT_OF_SCOPE]
- User-facing acceptance criteria: [ACCEPTANCE_CRITERIA]
- Risks or unknowns: [RISKS]
- Existing systems or dependencies: [DEPENDENCIES]
- Quality constraints: [PERFORMANCE / SECURITY / ACCESSIBILITY / RELIABILITY]
- Team context: [TEAM_CONTEXT]

This is a planning artifact only. Do not write code.

Required sections, in this exact order:
1. Planning Summary
2. Product Acceptance Criteria
3. Test Strategy
4. Test Cases Before Build
5. Fixture and Scenario Plan
6. Implementation Slices
7. Regression Risk Checklist
8. Definition of Done
9. Open Questions and Decision Gates

Strict TDD rules:
- No implementation step may appear before the tests that justify it.
- Every acceptance criterion must map to one or more planned tests.
- Edge cases and regressions must be planned, not deferred.
- If a criterion cannot be tested clearly, flag it as ambiguous.

Section requirements:
- "Test Strategy" must separate unit, integration, end-to-end, and non-happy-path coverage.
- "Test Cases Before Build" must use:
  Test name
  Intent
  Given
  When
  Then
- "Fixture and Scenario Plan" must define the minimum realistic setup needed to validate behavior.
- "Implementation Slices" must be thin vertical slices justified by the tests they unlock.
- "Definition of Done" must include passing tests, acceptance validation, and documentation updates.

Do not include code.
Do not use vague sequencing like "build backend then frontend."
The plan must be actionable enough that an engineer can start from the test list.
```

## 6. LUX-Specific Booster Prompt

Use this as an extra user-side block when the task is specifically about the existing LUX MVP set.

```text
Additional LUX constraints:
- Treat LUX as one operator product with three modules: client updates, leads, and content.
- Keep Slack, Airtable, JobTread, and Buffer as integrations, not the primary UI.
- Preserve the current human-in-the-loop operating model.
- Show source facts beside generated output wherever review accuracy matters.
- Do not flatten the three modules into one generic editor.
- Make ownership, status, and next action explicit on every screen.
- When proposing future implementation work, require strict TDD and no code output unless explicitly requested in a separate session.
```

## 7. Recommended Usage Order

Typical sequence:
1. Product UX Spec Prompt
2. User Flow Prompt
3. Wireframe Prompt
4. Visual Design Direction Prompt
5. Strict TDD Implementation Planning Prompt

This order keeps product decisions ahead of screens, and screens ahead of build planning.
