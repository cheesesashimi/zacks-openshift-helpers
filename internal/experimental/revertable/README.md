# revertable

This is an attempt to make a revertable execution queue. The idea behind this
is that one passes in a list of Revertable objects that can undo themselves. If
an error occurs while a Revertable object is applying itself, each object will
revert itself even when an error is encountered. All errors will be collected
and bubbled back up to the caller.
