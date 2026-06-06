# Togo4: On Street/Service Project
    Telegram long-polling bot version of TogoFor, for managing my todos.
    Includes scheduling, inline/reply keyboards, and SQLite persistence.
    This branch is the long-polling runtime (not the Vercel serverless variant).
# Project Properties:
* Language: golang
* Branches: This repository has 3 branches, each one is a different design and uses a special mechanism.
    * master / Togo4+ Bot: The final and ongoing variant of the bot.
        * Platform: Telegram
        * Mechasnism: Longpolling Bot,
        * Database: Sqlite3
        * Param Seperator: 2 Spaces. [Only]
        
    * ServerlessFunctionBot / Togo4 bot: Webhook like variant of the bot, running on vercel right now.
        * Platform: Telegram
        * Mechasnism: ServerlessFunction Bot [Vercel]
        * Database: Postgres
        * Downside: No Togo Schedular
        * Param Seperator: 2 Spaces. [Only]

    * ConsoleApp: The primary and console app version of the project. 
        * Platform: Obvious!
        * Database: Sqlite3
        * Param Seperator: tab

# Main changes vs previous togo4bot:
* Changed the mechanism from serverless function to longpolling.
* Added notification system for current togos, notifying users one minute before each togo start time.
# Link
    running on https://t.me/togo4plusbot

# Notes:
* Here command/param seperator is 2 SPACES (because telegram doesnt have a specific tab character)
* More than 2 spaces is still part of the arguments; Separator is Exactly 2 spaces; nothing more of less!
* Set these environmental variables in `.env` before startup:
TOKEN=telegram bot token
ADMIN_ID=telegram user id to receive admin notifications

# Markup Keyboard

Comparing to togo4 console app, this one has many extra features including a Reply Markup keyboard and Inline keyboards in many sections, making it easier to interact with the app.

# Commands

## `+` Add New Togo

Creates a new task with optional flags.

**Syntax:**
```
+  title  [=  weight]  [+p  progress]  [:  description]  [+x | -x]  [@  days  hh:mm]  [->  duration]
```

**Flags:**
- `=` or `+w` - Weight (importance, default: 1)
- `+p` - Progress percentage (0-100)
- `:` or `+d` - Description
- `+x` - Mark as extra task
- `-x` - Mark as normal task (default)
- `@` - Schedule (days from now, then time as HH:MM)
- `->` - Duration in minutes

**Examples:**
```
+  Buy groceries
+  Finish project  =  10  +p  50  :  Complete by Friday
+  Meeting  @  1  14:30
+  Workout  ->  60  +x
+  Report  =  8  :  Quarterly report  @  3  09:00
```

**Notes:**
- All flags are optional except title
- Flag order doesn't matter
- Flags and values must be separated by exactly 2 spaces

---

## `#` Show Togos

Display your tasks with optional filtering.

**Usage:**

| Command | Shows |
|---------|-------|
| `#` | Today's togos |
| `#  -` | Today's incomplete togos only |
| `#  +a` | All togos (all days) |
| `#  -a` | All incomplete togos (all days) |

**Examples:**
```
#
#  -
#  +a
#  -a
```

---

## `%` Show Progress

Calculate progress and completion statistics.

**Usage:**

| Command | Calculates |
|---------|------------|
| `%` | Today's progress |
| `%  -` | Today's progress (incomplete only) |
| `%  +a` | Overall progress (all days) |
| `%  -a` | Overall progress (incomplete only) |

**Examples:**
```
%
%  -
%  +a
%  -a
```

---

## `✅` Tick (Complete) Togos

Mark togos as complete/incomplete using inline buttons.

**Usage:**

| Command | Shows |
|---------|-------|
| `✅` | Today's togos (for ticking) |
| `✅  -a` | All togos for ticking |
| `✅  +a` | All togos for ticking |

**Examples:**
```
✅
✅  -a
```

Click any togo button to toggle its completion status.

---

## `❌` Remove Togos

Remove/delete togos using inline buttons.

**Usage:**

| Command | Shows |
|---------|-------|
| `❌` | Today's togos (for removal) |
| `❌  -a` | All togos for removal |
| `❌  +a` | All togos for removal |

**Examples:**
```
❌
❌  -a
```

Click any togo button to delete it.

---

## `tk` Quick Tick Togo

Toggle a togo's completion by id, without opening the inline keyboard.

**Syntax:**
```
tk  id
```

Toggles togo `#id` between done (100%) and not done (0%). Works for togos on any day.

**Examples:**
```
tk  1
tk  42
```

---

## `$` Get/Update Togo

Retrieve and update a specific togo by ID.

**Syntax:**
```
$  id  [=  weight]  [+p  progress]  [:  description]  [+x | -x]  [@  days  hh:mm]  [->  duration]
```

**Examples:**
```
$  1
$  1  =  5  +p  75
$  1  :  Updated description
```

---

## Chaining Commands

All commands can be chained in a single message. Use any command in sequence without prefix.

**Examples:**

```
+  New task  =  5  #  %
```
Creates a new task, shows today's togos, displays progress.

```
+  Task 1  #  -  +  Task 2  %
```
Creates two tasks, shows incomplete tasks, displays progress.

```
#  +p  50  $  1  :  Updated  #  +a
```
Shows today's togos, updates task 1, shows all togos.

---

## Other Notes

- All separators between command/flag and value must be exactly 2 spaces
- Commands can be combined in any order on a single line
- Each flag is case-sensitive (use lowercase)

## Command Token Reference

| Command | Token | Meaning |
|---------|-------|---------|
| `+` | (title) | Add new togo |
| `#` | (default) | Show today's togos |
| `#` | `-` | Show incomplete togos (today only) |
| `#` | `+a` | Show all togos (all days) |
| `#` | `-a` | Show all incomplete togos (all days) |
| `%` | (default) | Progress for today |
| `%` | `-` | Progress for incomplete togos (today) |
| `%` | `+a` | Progress for all togos (all days) |
| `%` | `-a` | Progress for all incomplete togos (all days) |
| `tk` | `id` | Toggle completion of togo by id (no keyboard) |
| `✅` | (default) | Tick/complete today's togos |
| `✅` | `-a` | Tick/complete all days' togos |
| `❌` | (default) | Remove today's togos |
| `❌` | `-a` | Remove all days' togos |

**Important:** All tokens require exactly 2 spaces as separator. Examples:
- ✅ Correct: `#  +a` (2 spaces between # and +a)
- ❌ Wrong: `#  + a` (treats +a as two separate terms)
- ❌ Wrong: `# +a` (only 1 space, won't parse correctly)
- ❌ Wrong: `#   +a` (3 spaces, won't parse correctly)

## Tasks (New Concept)

Tasks are separate from togos.

- No task deadline/time window/duration
- Optional start date (inactive until start date)
- Separate listing, progress stats, and reminder flow

### Task Commands

Add task (supports chaining in one message):

```bash
^  title  [=  weight]  [+p  progress]  [:  description]  [+x | -x]  [@  days_or_yyyy-mm-dd]
```

List active tasks:

```bash
~
```

List active + inactive tasks:

```bash
~  +i
```

Get/update task by id:

```bash
&  id  [=  weight]  [+p  progress]  [:  description]  [+x | -x]  [@  days_or_yyyy-mm-dd]
```

Tick tasks with inline keyboard:

```bash
✅T
```

Remove tasks with inline keyboard:

```bash
❌T
```

Quick tick a task by id (toggle done/undone) without the inline keyboard:

```bash
TK  id
```

Task-only progress:

```bash
%  t
```

Both togo + task progress in one report:

```bash
%  b
```

### Task Reminders

- Default: 4 times/day (every 6 hours)
- Supported values: `0, 1, 2, 4, 6, 8, 12, 24`
- `0` disables automatic task reminders

Set reminders/day:

```bash
~s  4
```

Show current reminder setting:

```bash
~s
```

### Task Pagination

Task list and reminder messages are automatically paginated when too long.

- Use inline `Next` / `Prev` buttons under the message
- Pagination callbacks refresh from current task data

### Inline Menu Pagination

The tick (`✅` / `✅T`) and remove (`❌` / `❌T`) inline keyboards are also
paginated. Telegram rejects keyboards with more than 100 buttons, so when a togo
or task list is large the buttons are split into pages of up to 90 items.

- A `⬅️ Prev` / `page/total` / `Next ➡️` row appears under the buttons
- Navigating re-loads the current togos/tasks, so the menu always reflects live data
- Ticking/removing an item keeps you on the same page

## Testing

### Run All Tests

```bash
go test ./...
```

This runs all unit tests and integration-style tests across the project.

### Run Specific Test Suites

Unit tests for core domain and DB logic:
```bash
go test -v ./Togo
```

Unit tests for test-stats parser tooling:
```bash
go test -v ./scripts/teststats
```

Integration-style parser/handler safety tests:
```bash
go test -v -run 'Handler|SplitArgumentsIntegration|ExtractRejectsAllTrailingFlags|ExtractAcceptsValidInput' .
```

B1 bounds checking tests specifically:
```bash
go test -v -run Handler integration_test.go main.go main_test.go
```

### Full Stats & Logs Script

```bash
./scripts/run_all_tests_with_stats.sh
```

This script runs four phases:
1. `go test -json ./...` and saves raw JSON events
2. `go test -coverprofile=... ./...` and captures coverage profile
3. `go tool cover -func` breakdown
4. `go run ./scripts/teststats` aggregation report

Artifacts are saved in a timestamped directory under `.test-logs/`:
- `go-test.jsonl`
- `coverage.out`
- `coverage.func.txt`
- `summary.txt`

### Test Stats Parser Tool

The parser behind the summary script lives at `scripts/teststats/main.go` and can be run directly:

```bash
go run ./scripts/teststats --json .test-logs/<run-id>/go-test.jsonl --coverage .test-logs/<run-id>/coverage.func.txt
```

It reports:
- Package pass/fail/other totals
- Test run/pass/fail/skip totals
- Wall-clock duration from test JSON events
- Top slowest tests
- Lowest covered files (average by function)

### Coverage Confidence Map (Current Scope)

| Project Area | Coverage Status | Test Location(s) |
|--------------|-----------------|------------------|
| Command parsing and keyboard/report helpers (`main.go`) | High (core helper/runtime paths covered) | `main_test.go`, `integration_test.go` |
| Domain parsing, DB CRUD, stats (`Togo/Togo.go`) | High | `Togo/Togo_test.go` |
| JSON/coverage aggregation tooling (`scripts/teststats/main.go`) | Medium-High | `scripts/teststats/main_test.go` |
| Long-running bot entry loop (`main.go:main`, perpetual scheduler loop wrapper) | Integration/runtime only | Manual run + bot runtime |
| Example/demo files (`ex/`) | Not targeted | N/A |

Notes:
- The highest-risk business logic now has direct tests.
- Remaining untested paths are runtime entrypoints or demo/example files, not core business rules.

### Build

```bash
go build ./...
```

Ensures no compilation errors and all dependencies are correct.

## P.S.

Street/Service Project means this one is coded while walking streets or while doing service!
