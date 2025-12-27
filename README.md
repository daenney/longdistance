# longdistance

A Go library for folks whose relationship status with Linked Data is "It's Complicated".

This library implements parts of the JSON-LD 1.0 and [JSON-LD 1.1][jld] specification. It was initially written to handle [ActivityStreams 2.0][as], but works for most JSON-LD documents. See the Features section for what does and does not work.

[jld]: https://www.w3.org/TR/json-ld/
[as]: https://www.w3.org/TR/activitystreams-core/

For each implemented functionality, it passes the associated [JSON-LD test suite][jldtest] provided by the W3C.

[jldtest]: https://w3c.github.io/json-ld-api/tests/

## Documentation

See the [godoc](https://pkg.go.dev/sourcery.dny.nu/longdistance).

## Features

A limited feature set of [JSON-LD Processing Algorithms and API specification][jldapi] is supported:
* Context processing.
  * Remote context retrieval is supported, but requires a loader to be provided.
* Document expansion.
* Document compaction.
    * Except `@preserve` since framing is not supported.

[jldapi]: https://www.w3.org/TR/json-ld11-api/#compaction-algorithm

Not supported:
* Document flattening.
* Framing.
* RDF serialisation/deserialisation.
* Remote document retrieval and extraction of JSON-LD script elements from HTML.

By not supporting some of these features, the internals of the library can remain fairly simple. Adding any of these features comes with significant complexity. If you're able and willing to contribute one of these features, please start by opening an issue so we can discuss how to appraoch it.

## License

This library is licensed under the Mozilla Public License Version 2.0.
