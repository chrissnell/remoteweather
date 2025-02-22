# Unit

Four types: Angle, HourAngle, RA, and Time, useful in astronomy applications.

These types are all angle-like types.  The Time type is at least angle-related.
It has conversions to and from the other types and has a function to wrap a
Time value to the fractional part of a day.

## Motivation

This package supports two other packages, github.com/soniakeys/sexagesimal and
github.com/soniakeys/meeus.  The sexagesimal package adds formatting to the
four types defined here.  The meeus package implements a large colection of
astronomy algorithms.

## Install

### Go get

Technically, `go get github.com/soniakeys/unit` is sufficient as usual.

The tests also require the sexagesimal package, so use the -t option to prompt
`go get` to find it as a test dependency:

    go get -t github.com/soniakeys/unit

### Vgo

Experimentally, you can try [vgo](https://research.swtch.com/vgo).

To run package tests, clone the repository -- anywhere! it doesn't have to
be under GOPATH -- and from the cloned directory run

    vgo test

Vgo will fetch the sexagesimal test dependency as needed and run the unit
package tests.

### Or don't install it

If you only need `unit` as dependency of some other package that you are
installing, the normal installation of that package will likely install `unit`
for you.  Try that first.

## License

MIT
