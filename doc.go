/* Package main: FIRST & THIRD -- almost FORTH

FORTH is a language mostly familiar to users of "small" machines. FORTH
programs are small because they are interpreted--a function call in FORTH takes
two bytes.  FORTH is an extendable language-- built-in primitives are
indistinguishable from user-defined _words_.  FORTH interpreters are small
because much of the system can be coded in FORTH--only a small number of
primitives need to be implemented.  Some FORTH interpreters can also compile
defined words into machine code, resulting in a fast system.

FIRST is an incredibly small language which is sufficient for defining the
language THIRD, which is mostly like FORTH.  There are some differences, and
THIRD is probably just enough like FORTH for those differences to be disturbing
to regular FORTH users.

The only existing FIRST interpreter is written in obfuscated C, and rings in at
under 800 bytes of source code, although through deletion of whitespace and
unobfuscation it can be brought to about 650 bytes.

This document FIRST defines the FIRST environment and primitives, with relevent
design decision explanations.  It secondly documents the general strategies we
will use to implement THIRD.  The THIRD section demonstrates how the complete
THIRD system is built up using FIRST.

Section 1: see first.go

Section 2: Motivating THIRD

What is missing from FIRST?  There are a large number of important primitives
that aren't implemented, but which are easy to implement.  drop , which throws
away the top of the stack, can be implemented as { 0 * + } -- that is, multiply
the top of the stack by 0 (which turns the top of the stack into a 0), and then
add the top two elements of the stack.

dup , which copies the top of the stack, can be easily implemented using
temporary storage locations.  Conveniently, FIRST leaves memory locations 3, 4,
and 5 unused.  So we can implement dup by writing the top of stack into 3, and
then reading it out twice: { 3 ! 3 @ 3 @ }.

We will never use the FIRST primitive 'pick' in building THIRD, just to show
that it can be done; 'pick' is only provided because pick itself cannot be
built out of the rest of FIRST's building blocks.

So, instead of worrying about stack primitives and the like, what else is
missing from FIRST?  We get recursion, but no control flow--no conditional
operations.  We cannot at the moment write a looping routine which terminates.

Another glaring dissimilarity between FIRST and FORTH is that there is no
"command mode"--you cannot be outside of a : definition and issue some straight
commands to be executed. Also, as we noted above, we cannot do comments.

FORTH also provides a system for defining new data types, using the words [in
one version of FORTH] <builds and does> . We would like to implement these
words as well.

As the highest priority thing, we will build control flow structures first.
Once we have control structures, we can write recursive routines that
terminate, and we are ready to tackle tasks like parsing, and the building of a
command mode.

By the way, location 0 holds the dictionary pointer, location 1 holds the
return stack pointer, and location 2 should always be 0--it's a fake dictionary
entry that means "pushint".

Section 3: see third.go

*/
package main
