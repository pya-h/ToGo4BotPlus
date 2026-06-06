# Feature Ideas / Backlog

A menu of possible future work, captured during the 2026-06-07 full-codebase review.
These are **suggestions only** — none are implemented. Roughly ordered by effort.

> Note: this is a product/feature wishlist. For the smaller code-level inconsistencies
> found during review (e.g. `@ N` schedule ambiguity, `Togo.isCommand` missing tokens,
> rolling-24h `LoadEverybodysToday`, 4096-char message cap), see the review notes — those
> are behavior-altering and intentionally left untouched.

## Quick wins
- [ ] **`/help` as a real slash command** + register it in `setMyCommands` (today help only
      appears on unknown input).
- [ ] **Idea → Togo/Task promotion**: a "⬆️ Promote" button on the idea card that opens the
      addTogo/addTask wizard pre-filled with the idea text. Closes the loop from
      brainstorming to doing.
- [ ] **Idea search / text filter** (e.g. `;  ?  keyword`) to find ideas by substring.
- [ ] **Edit-text validation** in the manage flow: reject empty idea text (currently an empty
      value is accepted and produces a blank idea).
- [ ] **Pagination for the manage picker** (`/ideas`, `/togos`, `/tasks`): the tick/remove
      inline menus paginate, but the manage list silently caps at 90 items.

## Medium
- [ ] **Recurring togos** (daily/weekly). The minute-by-minute notifier infrastructure already
      exists; this is arguably the highest-value productivity feature for a todo bot.
- [ ] **Snooze / reschedule** button on togo reminder notifications.
- [ ] **Tags/labels across all entities** — generalize the per-user idea-category mechanism into
      a shared labels table usable by togos and tasks too.
- [ ] **Per-user timezone** instead of the hardcoded `Asia/Tehran` (store it in a settings
      table; `task_settings` already exists as a model).
- [ ] **Weekly/daily digest** ("here's tomorrow's schedule") delivered via the existing scheduler.

## Larger / infrastructure
- [ ] **Single shared `*sql.DB` + WAL** instead of `sql.Open`+`Close` per call (perf, fewer file
      descriptors; complements the rows.Close() leak fix from the review).
- [ ] **Undo for deletes** — soft-delete column + an "↩️ Undo" button for ~10 seconds.
- [ ] **Export** (`/export` → CSV/JSON of a user's togos/tasks/ideas).
- [ ] **Inline-query mode** (`@yourbot  search`) to surface ideas/togos in any chat.
- [ ] **Graceful shutdown** (drain the updates channel, close the DB) + structured logging.
