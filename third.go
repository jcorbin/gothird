package main

import (
	"io"
	"strings"
)

const thirdSource = `

exit : immediate _read @ ! - * / <0 echo key pick

: r 1 exit

: ] r @ 1 - r ! _read ]

: main immediate r @ 7 ! ]
main

: _x  3 @ exit
: _x! 3 ! exit
: _y  4 @ exit
: _y! 4 ! exit

: swap _x! _y! _x _y exit

: + 0 swap - - exit

: dup _x! _x _x exit

: inc dup @ 1 + swap ! exit

: h 0 exit

: , h @ ! h inc exit

: ' r @ @ dup 1 + r @ ! @ exit

: ; immediate ' exit , exit

: drop 0 * + ;

: dec dup @ 1 - swap ! ;

: tor r @ @ swap r @ ! r @ 1 + r ! r @ ! ;
: fromr r @ @ r @ 1 - r ! r @ @ swap r @ ! ;

: tail fromr fromr drop tor ;

: minus 0 swap - ;
: bnot 1 swap - ;
: < - <0 ;

: logical dup 0 < swap minus 0 < + ;
: not logical bnot ;
: = - not ;

: branch r @ @ @ r @ @ + r @ ! ;
: computebranch 1 - * 1 + ;

: notbranch
  not
  r @ @ @
  computebranch
  r @ @ +
  r @ !
  ;

: here h @ ;

: if immediate ' notbranch , here 0 , ;
: then immediate dup here swap - swap ! ;

: find-) key ')' = not if tail find-) then ;

: ( immediate find-) ;

( we should be able to do FORTH-style comments now )

( this works as follows: ( is an immediate word, so it gets
  control during compilation.  Then it simply reads in characters
  until it sees a close parenthesis.  once it does, it exits.
  if not, it pops off the return stack--manual tail recursion. )

( now that we've got comments, we can comment the rest of the code! )

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

: nl '\n' echo exit

: printnum
  dup
  10 mod '0' +
  swap 10 / dup
  if
    printnum 0
  then
  drop echo
;

: .
  dup 0 <
  if
    '-' echo minus
  then
  printnum
  <sp> echo
;

: debugprint dup . nl ;

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
  <
  if
    r @ @ @ r @ @ + r @ ! exit          ( branch )
  then
  fromr 1 +
  fromr drop
  fromr drop
  tor
;

: loop immediate ' inci , here - , ;

: loopexit
  fromr drop            ( pop off our return address )
  fromr drop            ( pop off i )
  fromr drop            ( pop off the limit of i )
  ;                     ( and return to the caller's caller routine )

: :: ;                  ( :: is going to be a word that does ':' at runtime )

: fix-:: immediate 1 ' :: ! ; ( vmCodeDefine = 1 )
fix-::

( Override old definition of ':' with a new one that invokes ] )
: : immediate :: r dec ] ;

: execute
  dup not if            ( execute an exit on behalf of caller )
	fromr drop          ( pop up to the caller's return )
	8 !                 ( hacked unary drop of the exit code )
	;                   ( back to the caller's caller )
  then
  8 !                   ( write code into temporary region )
  ' exit 9 !            ( with a following exit )
  8 tor ;               ( jump into temporary region )

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

: [ immediate command ;

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
        " ;)" nl exit
  then
  . " ), "
  1 +
  tail _dump
;

: dump _dump ;

: # . nl ;

: var <build , does> ;
: constant <build , does> @ ;
: array <build allot does> + ;

: _welcome " Welcome to THIRD.
Ok.
" ;

[

_welcome
`

type _thirdKernel struct{}

func (_thirdKernel) Name() string { return "third" }

func (_thirdKernel) WriteTo(w io.Writer) (_ int64, err error) {
	var n int
	if sw, ok := w.(io.StringWriter); ok {
		n, err = sw.WriteString(thirdSource)
	} else {
		n, err = w.Write([]byte(thirdSource))
	}
	return int64(n), err
}

func (_thirdKernel) Open() io.Reader {
	return struct {
		_thirdKernel
		io.Reader
	}{Reader: strings.NewReader(thirdSource)}
}

var thirdKernel _thirdKernel
