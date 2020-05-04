package main

import (
	"io"
	"strings"
)

//go:generate go test -generate-third .

const _thirdSource = `
exit : immediate _read @ ! - * / <0 echo key pick

: r 1 exit

: ]
  r @
  1 -
  r !
  _read ]

: main immediate ] main

: rb 10 exit

: _x  3 @ exit
: _x! 3 ! exit

: + _x! 0 _x - - exit

: dup _x! _x _x exit

: '
  r @ @
  dup
  1 +
  r @ !
  @
  exit

: exec
  rb @ !
  rb @ r !
  exit

: _read] _read ]
: reboot immediate
  ' _read]
  exec
reboot

: _y  4 @ exit
: _y! 4 ! exit

: swap
  _x! _y! _x _y
  exit

: inc
  dup @
  1 +
  swap !
  exit

: h 0 exit

: ,
  h @
  !
  h inc
  exit

: ; immediate ' exit , exit

: drop 0 * + ;

: dec
  dup @
  1 -
  swap !
  ;

: tor
  r @ @
  swap
  r @ !
  r @ 1 + r !
  r @ !
  ;

: fromr
  r @ @
  r @ 1 - r !
  r @ @
  swap
  r @ !
  ;

: minus 0 swap - ;
: bnot 1 swap - ;
: < - <0 ;
: logical
	dup
	0 <
	swap minus
	0 <
	+
	;
: not logical bnot ;
: = - not ;

: branch
  r @ @
  @
  r @ @
  +
  r @ !
  ;
: computebranch 1 - * 1 + ;
: notbranch
  not
  r @ @ @
  computebranch
  r @ @ +
  r @ !
  ;

: here h @ ;

: if immediate
  ' notbranch ,
  here
  0 ,
  ;

: then immediate
  dup
  here swap -
  swap !
  ;

: tail fromr fromr drop tor ;

: find-)
  key
  ')' =
  not if
  tail find-)
  then ;
: ( immediate find-) ;

: else immediate
  ' branch ,            ( compile a definite branch )
  here                  ( push the backpatching address )
  0 ,                   ( compile a dummy offset for branch )
  swap                  ( bring old backpatch address to top )
  dup here swap -       ( calculate the offset from old address )
  swap !                ( put the address on top and store it )
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

: nl <nl> echo ;

: > swap < ;
: <= 1 + < ;
: >= swap <= ;

: i r @ 1 - @ ;
: j r @ 3 - @ ;

: do immediate
  ' swap ,              ( compile 'swap' to swap the limit and start )
  ' tor ,               ( compile to push the limit onto the return stack )
  ' tor ,               ( compile to push the start on the return stack )
  here                  ( save this address so we can branch back to it )
;

: inci
  r @ 1 -               ( get the pointer to i )
  inc                   ( add one to it )
  r @ 1 - @             ( find the value again )
  r @ 2 - @             ( find the limit value )
  <
  if
	r @ @ @
	r @ @ +
	r @ !
	exit          ( branch )
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
;                       ( and return to the caller's caller routine )

( :: is going to be a word that does ':' at runtime )
: :: ; ;
: fix-:: immediate 1 ' :: ! ; ( vmCodeDefine = 1 )
fix-::

( Override old definition of ':' with a new one that invokes ] )
: : immediate :: r dec ] ;

: _z  5 @ exit
: _z! 5 ! exit

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
  here _z!              ( store dict pointer in temp variable )
  _read                 ( compile a word )
                        ( if we get control back: )
  here _z
  = if
    tail command        ( we didn't compile anything )
  then
  here 1 - h !          ( decrement the dictionary pointer )
  here _z               ( get the original value )
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
`

type _thirdKernel struct{}

func (_thirdKernel) Name() string { return "third" }

func (_thirdKernel) WriteTo(w io.Writer) (_ int64, err error) {
	var n int
	if sw, ok := w.(io.StringWriter); ok {
		n, err = sw.WriteString(_thirdSource)
	} else {
		n, err = w.Write([]byte(_thirdSource))
	}
	return int64(n), err
}

func (_thirdKernel) Open() io.Reader {
	return struct {
		_thirdKernel
		io.Reader
	}{Reader: strings.NewReader(_thirdSource)}
}

var thirdKernel _thirdKernel
