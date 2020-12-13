# htmlparse

This package aims to parse HTML very quickly, at the cost of not being entirely spec-compliant.
The parse tree it generates will roughly match the input HTML file, rather than being rearranged according to spec.
It's also not particularly memory efficient, as it stores the entire input in memory rather than streaming it from an `io.Reader`.

The output uses [`*golang.org/x/net/html.Node`][node], allowing use of that package's rendering facilities, or of other packages that use the `Node` type, such as [vdom].

[node]: https://pkg.go.dev/golang.org/x/net/html#Node
[vdom]: https://github.com/vktec/vdom

## Performance

In my [testing], `htmlparse` outperforms `golang.org/x/net/html` by roughly a factor of 2.
Care is taken to avoid expensive standard library functions and to reduce the number of allocations wherever possible.

[testing]: https://github.com/vktec/htmlparse/blob/master/parser_test.go#L129

## Nonconformance

A very incomplete list of nonconforming things about this parser:

- Optional tags are not implicitly inserted
- Any element can be self-closing, not just void elements
- Foreign elements are not properly supported, they are simply treated as normal elements
- CDATA is parsed as a comment
- Many things are not strictly checked
