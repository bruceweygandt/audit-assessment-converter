# Audit CSV Import

A terminal UI that turns an auditor's spreadsheet (CSV) into an audit assessment file you can import into Paramify.

## What it does

Each CSV row fills in one requirement's **capability assurance step** in Paramify. You choose which CSV columns supply:

- **plan, result and remediation** (Plate rich text; several columns can be stacked into plan or result, each under a bold heading named after the column), and
- per-row **review status, method, step status, assigned, assurance owner and reviewer**, with values you set on the Details screen filling any blank cells.

The tool updates an assessment you downloaded from Paramify. It writes a JSON file; nothing is sent to Paramify until you import it.

## Requirements

- Go 1.25 or newer
- A terminal at least 130 columns wide to see the live preview (the tool works in narrower windows without it)

## Try it with the demo files

The `demo/` folder has practice files, so you can try the tool right after cloning. All their content is made up, and the emails are fake.

- `demo/demo-assessment-report.csv`: an auditor's report with 16 columns and 15 rows. It mixes `KSI-` and `FRR-` IDs, includes blank cells, and has one ID (`KSI-XYZ-01`) that isn't in the assessment.
- `demo/demo-audit-assessments.json`: a pretend Paramify download with two assessments. Merge the CSV into **Demo - FedRAMP 20x Phase One Pilot Assessment**; the output file will contain only that one.

Clone and run it:

```sh
git clone https://github.com/bruceweygandt/audit-assessment-converter.git
cd audit-assessment-converter
./run
```

The file pickers open in `demo/`. Don't import practice output into a real Paramify workspace.

## Workflow

1. In Paramify, **download audit assessments**. This gives you `audit-assessments.json`. Starting a brand-new assessment? Create it in Paramify first, then download.
2. Save the auditor's spreadsheet as a **CSV**. Excel files can't be selected; in Excel, use File → Save As → CSV UTF-8.
3. Run the tool (below) and pick the assessment to update.
4. In Paramify, **import** the JSON file the tool wrote.

The tool reads and writes the current Paramify format (`programRequirementAssessments` / `programRequirement`). Files in the older `projectRequirementAssessments` format are rejected with a message to download them again.

How the update works:

- **The output file holds only the assessment you pick.** The other assessments in the download are left out, so importing the file doesn't duplicate them in Paramify.
- **The assessment's own properties come from the download, unchanged.** Name, target, program, and every other assessment-level property (including any Paramify adds later) are copied exactly: same values, types and order. Only the requirements are rewritten. The tool never asks for these, so they always match what Paramify has.
- **Matched requirements:** each CSV row whose ID matches a requirement updates that requirement's review status, assurance owner and reviewer, and its first assurance step.
- **Blank values never erase existing ones.** Assurance Owner, Reviewer, Assigned, Plan, Result and Remediation keep their current values when both the CSV cell and the Details screen leave them blank.
- **Unmatched IDs** are added as new requirements and listed on the final screen. They usually mean an ID prefix still needs stripping.
- **Your download is not overwritten.** By default the result is saved next to it with the date and time in its name, e.g. `audit-assessments-merged-2026-10-20-1432.json`, so it can't be confused with the download or an earlier run.

## Usage

```sh
./run
```

Or using Go directly:

```sh
go run ./cmd/audit-csv-import
```

Each screen shows its step number, explains what it does, and describes every option. **Tab is the only key that continues to the next step**, on every screen; enter and the other keys only change things on the current screen. Most screens start with a sensible choice already selected, so you can often just press tab. There are 11 steps.

The CSV doesn't need to follow any template. Every column it has is listed on the ID, mapping, details and plan screens, however many there are; long lists scroll.

1. **Welcome:** what you'll need. Press tab to start.
2. **Select the CSV.** Move to the file and press tab. The picker opens in `example_files/` if that folder exists where you ran the tool, otherwise in `demo/`, otherwise in the current folder.
3. **Select your Paramify download**, the same way. The picker opens in the CSV's folder.
4. **Select the assessment to update.**
5. **Check the CSV:** a preview of the first rows.
6. **Pick the requirement ID column.** The most likely column is pre-selected. It's the column whose values match the most requirements in the assessment.
7. **Match IDs to Paramify:** strip prefixes such as `KSI-` (see below). The needed prefixes are filled in for you, and a live count shows how many CSV IDs match.
8. **Map columns**, one at a time with enter (see [Column mapping](#column-mapping)). Tab continues to the next step, keeping the remaining columns as listed; if any of them haven't been reviewed and have no checked suggestion, it warns once first.
9. **Assessment details:** review status, method, step status, assigned, assurance owner and reviewer (see [Which value wins](#which-value-wins)), plus the **Plan** fallback: optional plan text for rows whose plan columns are all blank. The assessment's name and program are shown at the top, taken from the download.
10. **Where to save:** the file name and folder, as two separate fields, with the complete path shown below them. The suggested name includes the date and time of the run, and the cursor starts on the name. Leave off `.json` and it's added for you.
11. **Review and create the file.** Values the tool can't read in review status, method or step status columns are flagged here. The file contains just the assessment you updated.

### Keys

| Key | Action |
|---|---|
| **tab** | Continue to the next step. It's the only key that does, on every screen. In the file pickers it continues with the highlighted file (or opens a highlighted folder); on the Review screen it creates the file; on the last screen it quits. |
| **↑/↓** | Move through lists and fields |
| **←/→** | Change an option, step through CSV columns on a **From column** row, or scroll the CSV preview |
| **enter** | Act within the screen, never leave it: open a folder in the file pickers, confirm a column's mapping and move to the next column, or move to the next field on the Details screen |
| **esc** | Back one step (on the mapping screen, back one column) |
| **← / backspace** | Up a folder in the file pickers |
| **[ / ]**, **PgUp/PgDn** | Preview another CSV row (only PgUp/PgDn while typing in a text box) |
| **ctrl+p** | Hide or show the live preview |
| **ctrl+c** | Quit from any screen. **q** also quits on the first, last and error screens, where nothing is lost. |

### Live preview

In a window at least 130 columns wide, the right side of the screen shows both files, live, from the ID column step onward:

- **Top: the CSV as a spreadsheet grid.** It has spreadsheet row numbers, the real header row, and a row under it showing where each column is mapped (`→ result`, `· skip`, …). The previewed row is marked ▶, and the column you're working on is highlighted.
- **Bottom: the JSON entry that row becomes**, exactly as it will be written to the output file. It's a line-by-line diff against your download: `-` red for what's removed and `+` green for what's added. The lines for the field you're working on are marked ▶, and the view scrolls to them.

Both update on every keypress. On the mapping screen, you see where a column lands *before* you confirm. On the Details screen, the field you're editing is lit up as you type. The preview runs the same merge code that writes the file, and never changes your download.

## Column mapping

Every client's spreadsheet is different, so the tool doesn't guess from column names alone. A column is pre-selected, and marked *(suggested)* with the evidence shown, only when its **contents** confirm it:

- **method / step status / review status:** at least 80% of the filled cells are exactly a known value (e.g. *Examine*, *Partial*, *Other Than Satisfied*, *Accepted*). Loose matches like *Implemented* don't count. Values that fit both statuses (like *Open*) are only suggested when the header says which (e.g. "Review Status", "Step Result").
- **assigned / owner / reviewer:** the header names the role *and* the cells are emails or short names.
- **result / plan / remediation:** never pre-selected. Their contents are free text, so you choose them.
- **Ambiguous columns aren't suggested.** If two columns qualify for the same field, neither is pre-selected.

| Target | What it fills | If the cell is blank |
|---|---|---|
| **result** | Step result, under a bold heading named after the column. Several columns stack. | Section left out. A row with no result at all keeps its existing result. |
| **plan** | Step plan, under a bold heading named after the column. Several columns stack. | Section left out; a row where every plan column is blank uses the plan text from the Details screen, or keeps its existing plan |
| **remediation** | Step remediation | The existing text is kept |
| **review status** | The requirement's Review Status, one of Paramify's eight: *Open*, *Overdue*, *Submitted*, *Accepted*, *Archived*, *Change Requested*, *QA Requested*, *Reviewed* (also *In progress* → OPEN, *In review* → SUBMITTED, *Approved* → ACCEPTED, *Rejected* → CHANGE_REQUESTED) | Review Status from the Details screen |
| **method** | Step method, one of Paramify's four: *Test*, *Examine*, *Interview* or *Unknown* (if several are listed, the first one wins) | Method from the Details screen |
| **step status** | Step status, one of Paramify's four: *Open/Pending/Not tested* → OPEN, *Pass/Satisfied/Met/Yes* → PASS, *Fail/Other than satisfied/Not met/No* → FAIL, *Partial* → PARTIAL | Step Status from the Details screen: *Open*, *Pass*, *Fail* or *Partial* |
| **assigned** | Assessor assigned to the step | Value from the Details screen; if that's blank too, the existing person is kept |
| **assurance owner** | The requirement's Assurance Owner (email or team name) | Same as assigned |
| **reviewer** | Requirement reviewer | Same as assigned |
| **skip** | Nothing | — |

Every target except **result** and **plan** takes a single column; choosing one for a second column moves it, and the screen says which column it replaces.

### Which value wins

On the **Assessment Details** screen, each of Review Status, Method, Step Status, Assigned, Assurance Owner and Reviewer has two rows:

```
Review Status
  From column:  ◀ "Review Status" ▶      12 rows read from this column
  If blank:     ◀ OPEN ▶                  3 rows use OPEN (2 blank, 1 not recognized)
```

- **The column wins.** Every row with a usable value in the **From column** gets that value.
- **The value under it is the fallback.** It's used only for rows where that cell is blank or can't be read. When no column is picked, the row is labelled **Every row** and the value applies to all of them.
- **The counts on the right** show what each row will get. They update as you change the column or the value, and the Review screen repeats them.
- **"Can be read" means an exact match.** For Review Status, Method and Step Status, a cell counts only if the whole cell is one of the values in the table above (capitals and punctuation don't matter). Sentences that merely contain a word like "met" or "test" are not read. They fall back to your value and are flagged on Review.
- **Assigned, Assurance Owner and Reviewer use any filled cell as-is.** If the picked column holds sentences rather than names or emails, the screen warns you, because each row's person would be set to that text. A dropdown column where no value can be read gets a "may be the wrong column" warning too.

Use ←/→ on a **From column** row to step through every CSV column, whatever it's named, or choose *none*. If a column already feeds another field, picking it moves it, and the screen says so. Browsing past a column doesn't change it; only the column you stop on changes. Plan and Result columns are picked on the mapping step (step 8), where several columns can be mapped to each; the Details screen's **Plan** group shows which plan columns were picked and holds the typed fallback text.

## ID prefix stripping

If your CSV has IDs like `KSI-CNA-01` but Paramify expects `CNA-01`, enter comma-separated prefixes to strip (e.g. `FRR-,KSI-`). Only the first matching prefix is removed from each ID. The screen previews the converted IDs as you type, and shows how many match the assessment and which don't.

## Testing

```sh
./test
```

Or using Go directly:

```sh
go test ./cmd/audit-csv-import
```

Some tests use real files and are skipped when the files aren't present. To run them, put these in `example_files/` at the repo root (the folder is gitignored, so client data isn't committed):

- `audit-assessments.json`: a download from Paramify. It checks that loading and re-saving changes nothing.
- `20x Phase Two Report - Paramify - 01.26.2026 - Human-Readable (1).csv` and `assessments.json`: the original 20x report fixtures.
