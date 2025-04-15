// Package longdistance can be used to process JSON-LD.
//
// You can turn incoming JSON into fully expanded JSON-LD using
// [Processor.Expand]. This will transform the document into a list of [Node].
// Each node has dedicated fields for each JSON-LD keyword, and the catch-all
// [Node.Properties] for everything else. If you serialise this document to JSON
// you'll get JSON-LD Expanded Document form.
//
// By calling [Processor.Compact] you can compact a list of [Node] to what looks
// like regular JSON, based on the provided compaction context. The result is
// serialised JSON that you can send out.
//
// By default a [Processor] cannot load remote contexts. You can install a
// [RemoteContextLoaderFunc] using [WithRemoteContextLoader] when creating the
// processor. You will need to provide your own. In order to not have
// dependencies on the network when processing documents, it's strongly
// recommended to create your own implementation of [RemoteContextLoaderFunc]
// with the necessary contexts built-in. You can take a look at the FileLoader
// in helpers_test.go.
//
// # JSON typing
//
// In order to provide a type-safe implementation, JSON scalars (numbers,
// strings, booleans) are not decoded and stored as [json.RawMessage] instead.
// You can use the optionally specified type to decide how to decode the value.
// When the type is unspecified, the following rules can be used:
//   - Numbers with a zero fraction and smaller than 10^21 are int64.
//   - Numbers with a decimal point or a value greater than 10^21 are float64.
//   - Booleans are booleans.
//   - Anything else is a string.
//
// Certain numbers might be encoded as strings to avoid size or precision issues
// with JSON number representation. They should have an accompanying type
// definition to explain how to interpret them. Certain strings might also hold
// a different value, like a timestamp or a duration. Those too should have a
// type specifying how to interpret them.
//
// # Constraints
//
// For JSON-LD, there are a few extra constraints on top of JSON:
//   - Do not use keys that look like a JSON-LD keyword: @+alpha characters.
//   - Do not use the empty string for a key.
//   - Keys must be unique.
package longdistance
