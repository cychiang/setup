---
name: pdf-reports
description: "Produce a structured PDF report with Typst that leads with a summary, cites every source with numbered references, embeds charts and diagrams, and rebuilds from source with one command. Use when the user asks for a report, a PDF, a write-up or an analysis document to share or print."
---

# PDF reports

## Structure

Write the report in this order:

1. **Title block.** Title, date, author or agent, and the data cutoff date.
2. **Summary.** The answer and the main findings in 3 to 6 sentences. A reader who stops here must know the result.
3. **Findings.** One section per finding. Each section states the finding, shows the evidence (a figure or table), and cites its sources.
4. **Method.** Data used, steps taken, assumptions, and known limits.
5. **References.** A numbered list of every source.

## Cite everything

- Every fact, number and claim that does not come from your own computation in this report gets a numbered citation, such as `[3]`.
- Each reference names where the fact came from: the document, the URL, the file path, the dataset and its retrieval date, or the page or section.
- Knowledge you used without a document behind it, such as a standard definition or model knowledge, still gets a reference that says so: "[5] General knowledge, not verified against a source."
- Numbers you computed cite the input data and the script that computed them.
- Never invent a source. If you cannot find one, label the claim as unverified in the text.

## Charts and diagrams

- Embed charts as SVG or PNG files produced by a script. Embed diagrams from Mermaid source rendered to SVG.
- Give each figure a caption that states the finding and the source, such as "Figure 2. Q3 revenue by region. Source [4]."
- Keep the data behind each chart in the report folder, and add a table of the key numbers when the reader needs exact values.
- If the `diagrams-and-charts` skill is installed, follow it for chart choice and labeling.

## Reproducible build

Keep everything for one report in one folder:

```
reports/<slug>/
  report.typ        # report source
  data/             # input data, or a script that fetches it
  figures/          # generated charts and diagrams
  scripts/          # scripts that produce figures from data
  README.md         # the build command and what it needs
```

Build with one command:

```
typst compile reports/<slug>/report.typ
```

This writes `report.pdf` next to the source. If figures come from scripts, put the full sequence in the README, such as `python scripts/figures.py && typst compile report.typ`, or a `Makefile` with one `make` target. Run the build yourself before you hand over the report, and open or inspect the PDF to confirm the figures rendered.

## Typst basics

```typst
#set document(title: "Q3 revenue review")
#set page(numbering: "1")
#set heading(numbering: "1.")

= Summary
Revenue grew 18 percent in Q3 [1].

#figure(
  image("figures/revenue.svg", width: 90%),
  caption: [Q3 revenue by region. Source [1].],
)

= References
+ Company 10-Q filing, Q3 2026, page 4. Retrieved 2026-11-20.
```

Typst can also manage citations from a `.bib` file with `#bibliography("refs.bib")` and `@key`. Use that when the report has more than about 10 sources.

## Fallback: pandoc

If Typst is not installed and cannot be installed, write `report.md` and build with:

```
pandoc report.md -o report.pdf --pdf-engine=typst
```

or with `--pdf-engine=xelatex` if a LaTeX engine is available. Keep the same structure and citation rules. State in the README which engine you used.

## Checks before you finish

- The summary states the answer.
- Every number and claim has a citation, and every reference says where the fact came from.
- Every figure has a caption with a source, and its data is in the folder.
- The one build command, run from a clean checkout, produces the PDF.
