---
name: concise-explaining
description: "Write explanations, answers and summaries that are short and easy to read. Lead with the answer, use plain words and the smallest structure that works. Use when answering a question, explaining code or a concept, summarizing results, or writing any reply or document section a person will read."
---

# Concise explaining

## Rules

1. **Answer first.** The first sentence answers the question or states the result. Put background and reasoning after it, and only as much as the reader needs to trust or use the answer.
2. **Plain words.** Write "use", not "leverage" or "utilize". Write "because", not "due to the fact that". Prefer the short common word.
3. **Smallest structure that works.**
   - One fact needs one sentence.
   - A few related points need a short paragraph.
   - Use a list only for parallel items or steps the reader follows in order.
   - Use a table only to compare several items on the same attributes.
   - Add headings only when the reply has sections a reader would jump between.
4. **Cut filler.** Delete openers ("Great question", "Sure"), restating the question, closing offers ("Let me know if"), hedges that add nothing, and summaries of what you just said.
5. **Define jargon once.** The first time you use a term the reader may not know, define it in a few words. Then use the same term every time. Do not switch between synonyms.
6. **Numbers with units.** Write "120 ms", "3.4 GB", "USD 1.2 million", never a bare number. Give the comparison base for a change: "from 80 to 120 ms (+50 percent)".
7. **One idea per sentence.** Split a sentence the reader would have to read twice.
8. **Concrete over abstract.** Name the file, the function, the command, the value. Replace "this improves performance" with the measured change.

## Before you send

- Does the first sentence answer the question?
- Can any sentence be deleted without losing information? Delete it.
- Does every number have a unit?
- Is any term used before it is defined?
- Would a list or table be shorter as one sentence? Use the sentence.

## Example

Question: "Why is the build slow?"

Too long: "Great question! There are several factors that can contribute to build slowness. After looking into it, it appears that one of the main issues may be related to dependency resolution..."

Concise: "The build spends 70 of its 90 seconds resolving dependencies, because the lockfile is missing. Commit `package-lock.json` and the build drops to about 20 seconds."
