**WIP**: An attempt to grok forth, by way of (re-)implementing [FIRST & THIRD
almost FORTH][first_and_third] purely by following its design document.

With a goal to have a useful forth VM core for further use once done.

Liberties taken:

- much broader literal parsing, including
  - quoted `rune` literals like `'A'`
  - control rune literals like `<ESC>`
  - alternate base integer literal parsing like `0xdead`
- a paged memory model, rather than one "big array"
- configurable return stack and memory base addresses
- builtin word inlining

[first_and_third]: http://www.ioccc.org/1992/buzzard.2.design
