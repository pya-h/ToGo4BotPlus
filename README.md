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
   Comparing to togo4 console app, this one has many extra features icluding a Reply Markup keyboard and Inline keyboards on many section,
   Making it easyier to interact with the app.
# Commands
# +: New Togo:
=> ... +   title   [=  weight]    [+p   progress_till_now]   [:   description]    [+x | -x]   [@  start_date_as_how_many_days_from_now    start_time_as_hh:mm]    [NEXT_COMMAND]

*   Flags order are optional, and Flags and their params must be seperated by 2 SPACES.
*   weight value can also be set by +w flag
*   description value can also be set by +d flag
# #: Show Togos
=> ...   #   [NEXT_COMMAND]
    by default shows today's togos
=> ...   #   -[NEXT_COMMAND]
    Show incompleted togos.
=> ...   #   +a   [NEXT_COMMAND]
    Show all togos on any day
=> ...   #   -a   [NEXT_COMMAND]
    Show all togos on any day, which are not completed yet.
# %: Progress Made:
=> ...   %   [NEXT_COMMAND]
    Calculate the progress been made (by default for Today)
=> ...   %   -[NEXT_COMMAND]
    Calculate the progress been made, just considering the incompleted and ongoing togos.
=> ...   %   +a  [NEXT_COMMAND]
    Calculate the progress been made, considering everything on any day.
=> ...   %   -a [NEXT_COMMAND]
    Calculate the progress been made considering all incompleted togos on any day.

# $: Get / Update a togo
=> ... $   id   [NEXT_COMMAND]
*   this will get and show a togo (just in today)
=> ... $   id   [=  weight]    [+p   progress_till_now]   [:   description]    [+x | -x]   [@  start_date_as_how_many_days_from_now    start_time_as_hh:mm]    [NEXT_COMMAND]

# Other Notes:
*   ... means that these cammands can also be used after previous command in the same line.
*   Each line can contain multiple command, as many as you want. Like:

=>   +   new_togo    @   1   10:00   +p  85  #  +   next_togo   +x  #   %

*   Extra:
=>        +x: its an extra Togo. its not mandatory but has extra points doing it.
=>        -x: not extra (default)
*   all params between [] are optional.


# Remember:
*   The flag list [& also commands] separator is 2 SPACES. space character will be evaluated as a part of the current flag's param. do not be mistaken.
*   in 'add new togo' syntax, all flags are optional except for the title, meaning that you can simply add new togos even with specifying the title only such as:
=>  +   new togo here
*   use a flag for % and # commands to expand the togos range to ALL.
*   use -a flag for % and # commands, to include All time togos, but only teh ones that are not done.

# P.S.:
   Street/Service Project, means that this one is coded while walking streets or while doing service!
