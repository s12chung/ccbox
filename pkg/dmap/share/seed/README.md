This directory is mounted to the container at `/home/ccbox/.ccbox/persist/` and is shared amongst all ccbox runs for the project.

On the host, it is located at `~/.ccbox/persist/<project>` only once a run changes this folder — an untouched folder leaves nothing behind.
