# longdistance

A Go library for folks whose relationship status with Linked Data is "It's Complicated".

This library implements parts of the [JSON-LD 1.1][jld] specification. It does not currently implement features from the JSON-LD 1.1 Processing Algorithms and API specification that are not needed for handling [ActivityStreams][as].

[jld]: https://www.w3.org/TR/json-ld/
[as]: https://www.w3.org/TR/activitystreams-core/

For each implemented functionality, it passes the associated [JSON-LD test suite][jldtest] provided by the W3C.

[jldtest]: https://w3c.github.io/json-ld-api/tests/

## Documentation

See the [godoc](https://pkg.go.dev/sourcery.dny.nu/longdistance).

## Supported features

* Context processing.
  * Remote context retrieval is supported, but requires a loader to be provided.
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
