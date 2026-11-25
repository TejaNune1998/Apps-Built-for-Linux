# TUI update

The TUI has been updated toward the proposed SPF-inspired retro layout.

## Changes

- Replaced the rounded Tokyo Night panels with classic single/double-line frames and terminal 16-colour ANSI styling.
- Added proportional 24/38/50 columns, a title/breadcrumb/clock row, status row, and F1-F10 shortcut bar.
- Disabled the alternate screen so the TUI renders inline.
- Added mouse click support for selecting panels and opening a details field, while retaining keyboard navigation.
- Added task status symbols, sub-task counts, recurrence marks, and overdue markers.
- Added Glamour rendering for `details.md`, with a scrollable details panel and scrollbar indicators.
- Moved task/sub-task editing to individual fields in the details panel. Priority, status, and recurrence use radio-style choice dialogs; date/time fields validate `YYYY-MM-DD HH:MM`.
- Kept the existing directory layout, `.tmp` files, work-log format, alert format, recurrence logic, CLI, and shared reminder script.
- Updated setup to run `go mod tidy` and build the TUI binary after copying the project.

## Build manually

```bash
cd scripts/tui
go mod tidy
go build -o to-do-tui .
```

The module requires Go 1.24.2 or newer. `setup.sh --install` now performs the dependency resolution and build automatically.

## Schedule, task tree, and date-picker update

- Replaced the Help panel with a Today schedule showing starts and due dates in chronological order, including overdue due items and sub-task markers.
- Added an Upcoming dialog covering the next 30 days, grouped by day. Selecting an entry switches to its label, expands its parent path, and opens its details.
- Changed the task list to an expandable tree and added a collapsed Completed & Canceled group at the end. Closed entries are read-only in the TUI.
- Updated the function-key map: F1 Add, F2 Sub-task, F3 Work log, F4 Done, F5 Cancel, F6 Delete label, F7 Upcoming, F8 Reload, F9 Help, F10 Quit. Main-screen letter shortcuts were removed; Esc and q quit from the main screen.
- Added a calendar and digital time picker for task start/due times and custom alerts. It supports day navigation, month navigation with PgUp/PgDn, hour/minute/AM-PM adjustment, Tab focus movement, and mouse selection.
- Changed generated `details.md` headings so the task title is the heading itself, and removed the `===` separator in both the CLI and TUI detail builders.
- Kept the shared reminder script and alert timestamp format unchanged.

## Screenshot-matched TUI pass

- Refined the centred edit, Help, Upcoming and date/time dialog layouts to follow the supplied reference screenshots.
- Changed the date/time picker to a side-by-side month calendar and digital clock.
- Updated the Help and Upcoming dialogs with aligned columns, grouped dates and selection styling.
- Removed the horizontal-rule separator from both CLI and TUI `details.md` generation.
- Kept the 16-colour terminal palette, inline rendering, task storage and reminder alert format.
- Pinned `github.com/charmbracelet/x/cellbuf` to `v0.0.15` for compatibility with `ansi v0.11.6`.
