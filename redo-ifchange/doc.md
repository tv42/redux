## redo-ifchange

The `redo-ifchange` command is used in a `.do` script. When the `.do` file for a target A contains the
line `redo-ifchange B`, this means that the target A depends on B and A should be rebuilt if B
changes. 

## Usage

    redo-ifchange TARGET [TARGETS...]

This command should be placed in a `.do` script and should not be run directly.

## Notes

Conceptually, redo-ifchange performs three tasks.

First, it creates a prerequisite record between A and B so A can track changes to B.
Second, it creates a dependency record between B and A so a change in B immediately invalidates  A.
Finally, if B is out of date, redo-ifchange ensures that B is made up to date.

B is considered out of date if it does not exist, is not in the database, is flagged as out of date, 
has been modified or any of its dependents are out of date. Obviously this process may recurse.