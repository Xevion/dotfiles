---
name: humanizing-prose
description: Use when writing or editing public-facing text (posts, comments, articles, replies) where AI-generated patterns would be detectable or off-putting to human readers.
---

# Humanizing Prose

## Overview

AI-generated text has fingerprints. Readers -- especially on HN, Reddit, and other technical
communities -- can detect it viscerally even when they can't name specific tells. This skill is a
reference and process for stripping those fingerprints from any text before it goes out.

**Core principle:** Sound like a person who thought about something and typed it out, not a language
model asked to explain something comprehensively.

## Step 1: Intake (skip for anything one paragraph or shorter)

Before writing or editing, ask for style context. Don't ask all of these robotically -- pick the
ones that matter for the specific piece and ask them together in one message:

- **Voice/tone**: How do you normally write in this context? (casual, dry, blunt, technical, sardonic?)
- **Opinions**: Do you have a specific take here? What do you actually think?
- **Platform**: Where is this going? (HN, Reddit, Twitter/X, blog, LinkedIn?) -- register differs significantly
- **Intentional imperfections**: Should I add natural-sounding roughness? (see Step 5)

For genuinely short pieces (a single reply, a sentence or two), skip the questions and calibrate
from context.

## Step 2: Banned Vocabulary

Ctrl+F (or mentally scan) for these before posting. Replace or delete:

**Top offenders** (empirically most-detected; 9-25x spike in AI-generated text):
`delve`, `delves`, `delving`, `tapestry`, `nuanced`, `crucial`, `pivotal`, `vital`,
`comprehensive`, `robust`, `leverage` (as verb), `foster`, `underscore`, `underscores`,
`showcasing`, `kaleidoscope`, `landscape` (figurative), `realm`, `testament`, `paradigm`,
`synergy`, `holistic`, `transformative`, `multifaceted`, `intricate`, `beacon`, `cornerstone`,
`embark`, `navigate` (figurative), `resonate`, `unprecedented`, `vibrant`, `profound`,
`groundbreaking`, `innovative`, `meticulous`, `seamless`, `dynamic` (as vague descriptor)

**Transition word tells** (especially as paragraph openers):
`Moreover`, `Furthermore`, `Additionally` -- use "also", "and", or nothing

**Filler qualifiers:**
`It's important to note that`, `It's worth noting that`, `It's worth mentioning`, `It's crucial to`

**Essay structure closers:**
`In conclusion`, `In summary`, `To summarize`, `At the end of the day`

**Scene-setting openers:**
`In today's fast-paced world`, `In the digital age`, `As we navigate`, `In an era characterized by`

**Significance inflation:**
`marks a pivotal moment`, `stands as a testament`, `a vital role`, `reflects broader`,
`setting the stage for`, `indelible mark`, `deeply rooted`, `evolving landscape`

**Typographic symbols** -- never generate these. They are strong AI tells and belong in word
processors, not plain text. Use ASCII alternatives:

| Symbol | Name | Use instead |
|--------|------|-------------|
| `—` | em dash | `, ` or `.` or `--` or rewrite the sentence |
| `–` | en dash | `-` or "to" for ranges |
| `…` | ellipsis | `...` |
| `"` `"` | smart double quotes | straight `"` |
| `'` `'` | smart apostrophe | straight `'` |
| `·` | middle dot | `-` or spell it out |

These symbols sneak in via autocorrect, word processors, or model defaults. Ctrl+F each one before
posting. Zero occurrences is the target.

## Step 3: Structural Patterns to Avoid

**Negative parallelism** -- the single most-cited structural tell:
> "It's not just X, it's Y" / "Not X. Not Y. Z." / "This isn't about X, it's about Y"

Replace with a direct statement of what it actually is.

**Em dash** -- do not use. It's a typographic tell and an AI default. Use a comma, a period, or
rewrite the sentence. See the symbol table above for the replacement rule.

**Punchy rule-of-three** -- standalone sentences used as rhetorical punch:
> "No warning. No dialog. No email."

Fine once. Suspicious in clusters. Cut in technical writing where it's stylistically out of place.

**Trailing significance phrases** -- "-ing" clauses appended to claim importance:
> "...contributing to the broader history of X" / "...creating a vibrant community within its borders"

Cut them. If the fact mattered, the fact itself made the point.

**Bullet lists with bold lead-ins for everything** -- reserve for genuine reference material:
> **Key point:** Explanation...

Replace with flowing sentences when the content is narrative or argumentative.

**Five-paragraph essay skeleton** -- intro restates what you'll say, outro restates what you said.
Cut both. Start with the actual point.

**Second-person walkthrough scenarios:**
> "Imagine you created an API key three years ago..."

Flags LLM origin in technical writing. Use direct statements instead.

## Step 4: Tone Calibration

**Contractions**: Use them. "It's", "don't", "we've", "you're". Formal absence of contractions in
casual register is a tell.

**Take a stance**: Don't hedge everything with "while some argue... others believe...". If there's
a view, say it. Real people have opinions.

**Vague attribution**: "Experts say" and "industry reports suggest" without a citation are red
flags. Name the expert or cut the claim entirely.

**Uniform positivity**: Real people have complaints, frustrations, things they find annoying. Let
that through if it's there.

**Register by platform**:
- HN: technical precision, directness, no puffery
- Reddit: casual is fine, humor works, strong opinions expected
- Twitter/X: fragment sentences, abbreviate, let thoughts be incomplete
- Blog: prose flows, transitions are earned, not inserted

## Step 5: Intentional Imperfections (only if asked in Step 1)

If the human asked for natural-sounding roughness, consider adding one or two of these per 200 words:

- **Comma splice** where a period would be "correct": "It works, it's just slow"
- **And/But sentence openers**: Start a sentence with "And" or "But"
- **Mid-thought pivot**: "...or actually, maybe that's backwards"
- **Inconsistent Oxford comma**: Don't apply it with mechanical uniformity
- **One colloquial word in a technical sentence**: breaks register just enough to feel human
- **Trailing incomplete thought**: letting a point stand without a tidy closing bow

Don't pile these in. A paragraph with five "imperfections" reads as engineered. One or two is enough.

## Edit Pass Checklist

Run this before posting:

- [ ] Ctrl+F banned vocabulary -- replaced or deleted
- [ ] No "not X, but Y" / "not just X, it's Y" constructions
- [ ] No typographic symbols -- Ctrl+F for `—` `–` `…` `"` `"` `'` `'` `·` and replace with ASCII
- [ ] Paragraph openers -- no "Moreover", "Furthermore", "Additionally"
- [ ] No trailing "-ing" significance appendages
- [ ] Sentence length varies -- not all medium, not rhythmically identical
- [ ] At least one contraction (unless register genuinely demands formality)
- [ ] All attributions are specific ("Knuth says") or deleted ("experts say" is not an attribution)
- [ ] No "in conclusion" closer
- [ ] Read it aloud -- does any sentence sound like it came from a help article or product FAQ?
