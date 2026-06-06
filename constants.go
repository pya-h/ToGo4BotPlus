package main

const (
	MaximumInlineButtonTextLength = 24
	MaximumNumberOfRowItems       = 3
	NumberOfSeparatorSpaces       = 2
	MaximumTaskMessageLength      = 3200
	TaskReminderWarningPrefix     = "Task reminder warning:"
	// MaximumInlineMenuItems caps how many togo/task buttons appear on a single
	// inline keyboard page. Telegram rejects inline keyboards with more than 100
	// buttons, so we page well under that and reserve a row for navigation.
	MaximumInlineMenuItems = 90
)

const (
	TaskAddCommand           = "^"
	TaskListCommand          = "~"
	TaskUpdateCommand        = "&"
	TaskTickCommand          = "✅T"
	TaskRemoveCommand        = "❌T"
	TaskSettingsCommand      = "~s"
	TaskIncludeInactiveToken = "+i"
	TaskStatsToken           = "t"
	TaskBothStatsToken       = "b"
	TogoTickByIdCommand      = "tk"
	TaskTickByIdCommand      = "TK"
)

const HELP_MESSAGE = "WTF?\n```\n" +
	`## Commands
## +: New Togo:
=> +     title     [=  weight]      [+p     progress_till_now]     [:     description]      [+x | -x]     [@  start_date_as_how_many_days_from_now      start_time_as_hh:mm]      [...]

*     Flags order are optional, and Flags and their params must be seperated by 2 SPACES.
*     weight value can also be set by +w flag
*     description value can also be set by +d flag
## #: Show Togos
=>     #     [...]
      
	by default shows today's togos

=>     #     -     [...]
      
	Show incompleted togos.

=>     #     +a  [...] 
      
	Show all togos on any day

=>     #     -a     [...]
      
	Show all togos on any day, which are not completed yet.


## %: Progress Made:
=>     %     [...]
      
	Calculate the progress been made (by default for Today)

=>     %     -      [...]
      
	Calculate the progress been made, just considering the incompleted and ongoing togos.

=>     %     +a      [...]
      
	Calculate the progress been made, considering everything on any day.

=>     %     -a      [...]
      
	Calculate the progress been made considering all incompleted togos on any day.

## $: Get / Update a togo
=> $     id      [...]

     this will get and show a togo (just in today)

=> $     id     [=  weight]      [+p     progress_till_now]     [:     description]      [+x | -x]     [@  start_date_as_how_many_days_from_now      start_time_as_hh:mm]      [...]

## tk: Quick tick a togo by id (toggle done/undone)
=> tk     id      [...]

     Toggles the completion of togo #id without opening the inline keyboard.

## Other Notes:
*     [...] means that Bot supports chaining commands; You can chain any count of any of these commands and bot will do them in queue.
*     Each line can contain multiple command, as many as you want. Like:

=>     +     new_togo      @     1     10:00     +p  85  #  +     next_togo     +x  #   %

*   Extra:
=>        +x: its an extra Togo. its not mandatory but has extra points doing it.
=>        -x: not extra (default)
*   all params between [] are optional.


## Notes:
*   The flag list [& also commands] separator is 2 SPACES. space character will be evaluated as a part of the current flag's param. do not be mistaken.
*   in 'add new togo' syntax, all flags are optional except for the title, meaning that you can simply add new togos even with specifying the title only such as:
=>  +   new togo here
*   use a flag for % and # commands to expand the togos range to ALL.
*   use -a flag for % and # commands, to include All time togos, but only teh ones that are not done.


## Tasks (separate from togos):
=>     ^     title     [=  weight]      [+p     progress_till_now]     [:     description]      [+x | -x]     [@  start_date_as_how_many_days_from_now | yyyy-mm-dd]      [...]

	Add task (no task time-of-day)

=>     ~     [...]

	Show active tasks.

=>     ~     +i     [...]

	Show active + inactive tasks.

=>     &     id      [...]

	Get / update one task by id.

=>     ✅T

	Tick tasks using inline keyboard.

=>     ❌T

	Remove tasks using inline keyboard.

=>     TK     id

	Quick tick a task by id (toggle done/undone) without the inline keyboard.

=>     ~s

	Show current reminders/day setting.

=>     ~s     [0|1|2|4|6|8|12|24]

	Set reminders/day for task reminders. 0 disables reminders.

=>     %     t

	Show task-only progress.

=>     %     b

	Show combined togo + task progress.

*   Task reminder default is 4 times/day.` + "\n```"
