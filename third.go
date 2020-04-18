package main

import (
	"bytes"
	"io"
	"strings"
)

//// Section 3: Building THIRD

var thirdKernel = thirdSource{}

type thirdSource struct{}

func (thirdSource) Name() string { return "third.fs" }

// In this section, I'm going to keep my conversation out here, rather than
// using fake comments-- because we'll have real comments eventually.
func (thirdSource) WriteTo(w io.Writer) (n int64, err error) {
	flush := func(wto io.WriterTo) {
		if err != nil {
			return
		}
		var m int64
		m, err = wto.WriteTo(w)
		n += m
	}

	var buf bytes.Buffer
	line := func(parts ...string) {
		if err == nil {
			for _, s := range parts {
				buf.WriteString(s)
			}
			buf.WriteByte('\n')
			flush(&buf)
		}
	}

	// The first thing we have to do is give the symbols for our built-ins.
	line(`: immediate _read @ ! - * / <0 exit echo key pick`)

	// Next we want to be mildly self commenting, so we define the word 'r' to
	// push the *address of the return stack pointer* onto the stack--NOT the
	// value of the return stack pointer.  (In fact, when we run r, the value
	// of the return stack pointer is temporarily changed.)
	line(`: r 1 exit`)

	// Next, we're currently executing a short loop that contains _read and
	// recursion, which is slowly blowing up the return stack.  So let's define
	// a new word, from which you can never return.  What it does is drops the
	// top value off the return stack, calls _read, then calls itself.  Because
	// it kills the top of the return stack, it can recurse indefinitely.
	line(`: ]`,
		` r @`,   // Get the value of the return stack pointer
		` 1 -`,   // Subtract one
		` r !`,   // Store it back into the return stack pointer
		` _read`, // Read and compile one word
		` ]`)     // Start over

	// Notice that we don't need to exit, since we never come back. Also, it's
	// possible that an immediate word may get run during _read, and that _read
	// will never return!

	// Now let's get compile running.
	line(`: main immediate ]`)
	line(`main`)

	// NOTE inserted to reset the return stack
	// TODO also clear the data stack
	line(`: rb 10 exit`)
	line(`: reboot immediate rb @ r ! ]`)
	line(`reboot`)

	// Next off, I'm going to do this the easy but non-portable way, and put
	// some character constant definitions in. I wanted them at the top of the
	// file, but that would have burned too much of the return stack.
	line(`: '"'     34 exit`)
	line(`: ')'     41 exit`)
	line(`: '\n'    10 exit`)
	line(`: 'space' 32 exit`)
	line(`: '0'     48 exit`)
	line(`: '-'     45 exit`)
	line(`: cr '\n' echo exit`)

	// Next, we want to define some temporary variables for locations
	// 3, 4, and 5, since this'll make our code look clearer.
	line(`: _x  3 @ exit`)
	line(`: _x! 3 ! exit`)
	line(`: _y  4 @ exit`)
	line(`: _y! 4 ! exit`)

	// Ok.  Now, we want to make THIRD look vaguely like FORTH, so we're going
	// to define ';'.  What ; ought to do is terminate a compilation, and turn
	// control over to the command-mode handler. We don't have one, so all we
	// want ';' to do for now is compile 'exit' at the end of the current word.
	// To do this we'll need several other words.

	// Swap by writing out the top two elements into temps, and then reading
	// them back in the other order.
	line(`: swap _x! _y! _x _y exit`)
	// Take another look and make sure you see why that works, since it LOOKS
	// like I'm reading them back in the same order--in fact, it not only looks
	// like it, but I AM!

	// Addition might be nice to have.  To add, we need to negate the top
	// element of the stack, and then subtract. To negate, we subtract from 0.
	line(
		`: +`,
		` 0 swap -`,
		` -`,
		` exit`)

	// Create a copy of the top of stack
	line(`: dup _x! _x _x exit`)

	// Get a mnemonic name for our dictionary pointer--we need to compile
	// stuff, so it goes through this.
	line(`: h 0 exit`)

	// We're going to need to advance that pointer, so let's make a generic
	// pointer-advancing function. Given a pointer to a memory location,
	// increment the value at that memory location.
	line(`: inc`,
		` dup @`,  // Get another copy of the address, and get the value
		``,        // so now we have value, address on top of stack.
		` 1 +`,    // Add one to the value
		` swap`,   // Swap to put the address on top of the stack
		` ! exit`) // Write it to memory

	// , is a standard FORTH word.  It should write the top of stack into the
	// dictionary, and advance the pointer
	line(`: ,`,
		` h @`,   // Get the value of the dictionary pointer
		` !`,     // Write the top of stack there
		` h inc`, // And increment the dictionary pointer
		` exit`)

	// ' is a standard FORTH word.  It should push the address of the word that
	// follows it onto the stack.  We could do this by making ' immediate, but
	// then it'd need to parse the next word. Instead, we compile the next word
	// as normal.  When ' is executed, the top of the return stack will point
	// into the instruction stream immediately after the ' .  We push the word
	// there, and advance the return stack pointer so that we don't execute it.
	line(`: '`,
		` r @`,   // Get the address of the top of return stack
		``,       // We currently have a pointer to the top of return stack
		` @`,     // Get the value from there
		``,       // We currently have a pointer to the instruction stream
		` dup`,   // Get another copy of it--the bottom copy will stick
		``,       // around until the end of this word
		` 1 +`,   // Increment the pointer, pointing to the NEXT instruction
		` r @ !`, // Write it back onto the top of the return stack
		``,       // We currently have our first copy of the old pointer
		``,       // to the instruction stream
		` @`,     // Get the value there--the address of the "next word"
		` exit`)

	// Now we're set.  ; should be an immediate word that pushes the address of
	// exit onto the stack, then writes it out.
	line(`: ; immediate`,
		` ' exit`, // Get the address of exit
		` ,`,      // Compile it
		` exit`)   // And we should return

	// Now let's test out ; by defining a useful word:
	line(`: drop 0 * + ;`)

	// Since we have 'inc', we ought to make 'dec':
	line(`: dec dup @ 1 - swap ! ;`)

	// Our next goal, now that we have ;, is to implement if-then.  To do this,
	// we'll need to play fast and loose with the return stack, so let's make
	// some words to save us some effort.

	// First we want a word that pops off the top of the normal stack and
	// pushes it on top of the return stack.  We'll call this 'tor', for
	// TO-Return-stack.   It sounds easy, but when tor is running, there's an
	// extra value on the return stack--tor's return address!  So we have to
	// pop that off first...  We better just bite the bullet and code it
	// out--but we can't really break it into smaller words, because that'll
	// trash the return stack.
	line(`: tor`,
		` r @ @`,       // Get the value off the top of the return stack
		` swap`,        // Bring the value to be pushed to the top of stack
		` r @ !`,       // Write it over the current top of return stack
		` r @ 1 + r !`, // Increment the return stack pointer--but can't use inc
		` r @ !`,       // Store our return address back on the return stack
		` ;`)

	// Next we want the opposite routine, which pops the top of the return
	// stack, and puts it on the normal stack.
	line(`: fromr`,
		` r @ @`,       // Save old value
		` r @ 1 - r !`, // Decrement pointer
		` r @ @`,       // Get value that we want off
		` swap`,        // Bring return address to top
		` r @ !`,       // Store it and return
		` ;`)

	// Now, if we have a routine that's recursing, and we want to be polite
	// about the return stack, right before we recurse we can run { fromr drop
	// } so the stack won't blow up.  This means, though, that the first time
	// we enter this recursive routine, we blow our *real* return address--so
	// when we're done, we'll return up two levels. To save a little, we make
	// 'tail' mean { fromr drop }; however, it's more complex since there's a
	// new value on top of the return stack.
	line(`: tail fromr fromr drop tor ;`)

	// Now, we want to do 'if'.  To do this, we need to convert values to
	// boolean values.  The next few words set this up.

	// minus gives us unary negation.
	line(`: minus 0 swap - ;`)

	// If top of stack is boolean, bnot gives us inverse
	line(`: bnot 1 swap - ;`)

	// To compare two numbers, subtract and compare to 0.
	line(`: < - <0 ;`)

	// logical turns the top of stack into either 0 or 1.
	line(`: logical   `,
		` dup`,        // Get two copies of it
		` 0 <`,        // 1 if < 0, 0 otherwise
		` swap minus`, // Swap number back up, and take negative
		` 0 <`,        // 1 if original was > 0, 0 otherwise
		` +`,          // Add them up--has to be 0 or 1!
		` ;`)

	// not returns 1 if top of stack is 0, and 0 otherwise
	line(`: not logical bnot ;`)

	// We can test equality by subtracting and comparing to 0.
	line(`: = - not ;`)

	// Just to show how you compute a branch:  Suppose you've compiled a call
	// to branch, and immediately after it is an integer constant with the
	// offset of how far to branch. To branch, we use the return stack to read
	// the offset, and add that on to the top of the return stack, and return.
	line(`: branch`,
		` r @`,   // Address of top of return stack
		` @`,     // Our return address
		` @`,     // Value from there: the branch offset
		` r @ @`, // Our return address again
		` +`,     // The address we want to execute at
		` r @ !`, // Store it back onto the return stack
		` ;`)

	// For conditional branches, we want to branch by a certain amount if true,
	// otherwise we want to skip over the branch offset constant--that is,
	// branch by one.  Assuming that the top of the stack is the branch offset,
	// and the second on the stack is 1 if we should branch, and 0 if not, the
	// following computes the correct branch offset.
	line(`: computebranch 1 - * 1 + ;`)

	// Branch if the value on top of the stack is 0.
	line(`: notbranch`,
		` not`,
		` r @ @ @`,       // Get the branch offset
		` computebranch`, // Adjust as necessary
		` r @ @ +`,       // Calculate the new address
		` r @ !`,         // Store it
		` ;`)

	// here is a standard FORTH word which returns a pointer to the current
	// dictionary address--that is, the value of the dictionary pointer.
	line(`: here h @ ;`)

	// We're ALL SET to compile if...else...then constructs! Here's what we do.
	// When we get 'if', we compile a call to notbranch, and then compile a
	// dummy offset, because we don't know where the 'then' will be.  On the
	// *stack* we leave the address where we compiled the dummy offset. 'then'
	// will calculate the offset and fill it in for us.
	line(`: if immediate`,
		` ' notbranch ,`, // Compile notbranch
		` here`,          // Save the current dictionary address
		` 0 ,`,           // Compile a dummy value
		` ;`)

	// then expects the address to fixup to be on the stack.
	line(`: then immediate`,
		` dup`,    // Make another copy of the address
		` here`,   // Find the current location, where to branch to
		` swap -`, // Calculate the difference between them
		` swap !`, // Bring the address to the top, and store it.
		` ;`)

	// Now that we can do if...then statements, we can do some parsing!  Let's
	// introduce real FORTH comments. find-) will scan the input until it finds
	// a ), and exit.
	line(`: find-)`,
		` key`,         // Read in a character
		` ')' =`,       // Compare it to close parentheses
		` not if`,      // If it's not equal
		` tail find-)`, // repeat (popping R stack)
		` then`,        // Otherwise branch here and exit
		` ;`)

	flush(strings.NewReader(`
: ( immediate find-) ;

( we should be able to do FORTH-style comments now )

( now that we've got comments, we can comment the rest of the code
  in a legitimate [self parsing] fashion.  Note that you can't
  nest parentheses... )

: else immediate
  ' branch ,            ( compile a definite branch )
  here                  ( push the backpatching address )
  0 ,                   ( compile a dummy offset for branch )
  swap                  ( bring old backpatch address to top )
  dup here swap -       ( calculate the offset from old address )
  swap !                ( put the address on top and store it )
;

: over _x! _y! _y _x _y ;

: add
  _x!                   ( save the pointer in a temp variable )
  _x @                  ( get the value pointed to )
  +                     ( add the increment from on top of the stack )
  _x !                  ( and save it )
;

: allot h add ;

: maybebranch
  logical               ( force the TOS to be 0 or 1 )
  r @ @ @               ( load the branch offset )
  computebranch         ( calculate the condition offset [either TOS or 1])
  r @ @ +               ( add it to the return address )
  r @ !                 ( store it to our return address and return )
;

: mod _x! _y!           ( get x then y off of stack )
  _y _y _x / _x *       ( y - y / x * x )
  -
;

: printnum
  dup
  10 mod '0' +
  swap 10 / dup
  if
    printnum
    echo
  else
    drop
    echo
  then
;

: .
  dup 0 <
  if
    '-' echo minus
  then
  printnum
  'space' echo
;

: debugprint dup . cr ;

( the following routine takes a pointer to a string, and prints it,
  except for the trailing quote.  returns a pointer to the next word
  after the trailing quote )

: _print
  dup 1 +
  swap @
  dup '"' =
  if
    drop exit
  then
  echo
  tail _print
;

: print _print ;

( print the next thing from the instruction stream )
: immprint
  r @ @
  print
  r @ !
;

: find-"
  key dup ,
  '"' =
  if
    exit
  then
  tail find-"
;

: " immediate
  key drop
  ' immprint ,
  find-"
;

: do immediate
  ' swap ,              ( compile 'swap' to swap the limit and start )
  ' tor ,               ( compile to push the limit onto the return stack )
  ' tor ,               ( compile to push the start on the return stack )
  here                  ( save this address so we can branch back to it )
;

: i r @ 1 - @ ;
: j r @ 3 - @ ;

: > swap < ;
: <= 1 + < ;
: >= swap <= ;

: inci
  r @ 1 -               ( get the pointer to i )
  inc                   ( add one to it )
  r @ 1 - @             ( find the value again )
  r @ 2 - @             ( find the limit value )
  <=
  if
    r @ @ @ r @ @ + r @ ! exit          ( branch )
  then
  fromr 1 +
  fromr drop
  fromr drop
  tor
;

: loop immediate ' inci @ here - , ;

: loopexit
  fromr drop            ( pop off our return address )
  fromr drop            ( pop off i )
  fromr drop            ( pop off the limit of i )
;                       ( and return to the caller's caller routine )

: execute
  8 !
  ' exit 9 !
  8 tor
;

: :: ;                  ( :: is going to be a word that does ':' at runtime )

: fix-:: immediate 3 ' :: ! ;
fix-::

( Override old definition of ':' with a new one that invokes ] )
: : immediate :: ] ;

: command
  here 5 !              ( store dict pointer in temp variable )
  _read                 ( compile a word )
                        ( if we get control back: )
  here 5 @
  = if
    tail command        ( we didn't compile anything )
  then
  here 1 - h !          ( decrement the dictionary pointer )
  here 5 @              ( get the original value )
  = if
    here @              ( get the word that was compiled )
    execute             ( and run it )
  else
    here @              ( else it was an integer constant, so push it )
    here 1 - h !        ( and decrement the dictionary pointer again )
  then
  tail command
;

: make-immediate        ( make a word just compiled immediate )
  here 1 -              ( back up a word in the dictionary )
  dup dup               ( save the pointer to here )
  h !                   ( store as the current dictionary pointer )
  @                     ( get the run-time code pointer )
  swap                  ( get the dict pointer again )
  1 -                   ( point to the compile-time code pointer )
  !                     ( write run-time code pointer on compile-time pointer )
;

: <build immediate
  make-immediate        ( make the word compiled so far immediate )
  ' :: ,                ( compile '::', so we read next word )
  2 ,                   ( compile 'pushint' )
  here 0 ,              ( write out a 0 but save address for does> )
  ' , ,                 ( compile a push that address onto dictionary )
;

: does> immediate
  ' command ,           ( jump back into command mode at runtime )
  here swap !           ( backpatch the build> to point to here )
  2 ,                   ( compile run-code primitive so we look like a word )
  ' fromr ,             ( compile fromr, which leaves var address on stack )
;

: _dump                 ( dump out the definition of a word, sort of )
  dup " (" . " , "
  dup @                 ( save the pointer and get the contents )
  dup ' exit
  = if
        " ;)" cr exit
  then
  . " ), "
  1 +
  tail _dump
;

: dump _dump ;

: # . cr ;

: var <build , does> ;
: constant <build , does> @ ;
: array <build allot does> + ;

: [ immediate command ;

: _welcome " Welcome to THIRD.
Ok.
" ;

: ; immediate ' exit , command exit

[

_welcome

`))

	return
}
