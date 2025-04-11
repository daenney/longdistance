# longdistance

This is a Go library for folks whose relationship status with Linked Data is "It's Complicated". It implements parts of the [JSON-LD 1.1][jld] specification.

It does not currently implement features from the JSON-LD 1.1 Processing Algorithms and API specification that are not needed for handling [ActivityStreams][as].

[jld]: https://www.w3.org/TR/json-ld/
[as]: https://www.w3.org/TR/activitystreams-core/

For each implemented functionality, it passes the associated [JSON-LD test suite][jldtest] provided by the W3C.

[jldtest]: https://w3c.github.io/json-ld-api/tests/

## Documentation

See the [godoc](https://pkg.go.dev/code.dny.dev/longdistance).

## Supported features

* Context processing.
* Document expansion.
* Document compaction.
    * Except `@preserve`.

## Unsupported features

* Document flattening.
* Framing.
* RDF serialisation/deserialisation.
* Remote document retrieval.

If you're able and willing to contribute one of these features, please start by opening an issue so we can discuss how to appraoch it.

## License

This library is licensed under the Mozilla Public License Version 2.0.
