# Togo4: On Street/Service Project
    Telegram bot (By webhook) version of TogoFor. for managing my todos, in order to make me go for them.
    With many extra features and Memory/Performance & Coding optimization.
    This bot application is running on Vercel as a Serverless Function bot.
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
* Set these Environmental Variables for start:
TELEGRAM_TOKEN=token
POSTGRES_URL=postgres connection string

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
| `✅` | (default) | Tick/complete today's togos |
| `✅` | `-a` | Tick/complete all days' togos |
| `❌` | (default) | Remove today's togos |
| `❌` | `-a` | Remove all days' togos |

**Important:** All tokens require exactly 2 spaces as separator. Examples:
- ✅ Correct: `#  +a` (2 spaces between # and +a)
- ❌ Wrong: `#  + a` (treats +a as two separate terms)
- ❌ Wrong: `# +a` (only 1 space, won't parse correctly)
- ❌ Wrong: `#   +a` (3 spaces, won't parse correctly)

## Testing

### Run All Tests

```bash
go test ./...
```

This runs all unit tests and integration tests across the project.

### Run Specific Test Suites

Unit tests for Togo package:
```bash
go test -v ./Togo
```

Integration tests (bounds checking, panic recovery, etc.):
```bash
go test -v -run Integration
```

B1 bounds checking tests specifically:
```bash
go test -v -run Handler integration_test.go main.go main_test.go
```

### Build

```bash
go build ./...
```

Ensures no compilation errors and all dependencies are correct.

## P.S.

Street/Service Project means this one is coded while walking streets or while doing service!
