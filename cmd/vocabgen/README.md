# vocabgen

This is a small CLI that given an input context and a document IRI will generate Go consts for all the terms. This is very best-effort and especially when property or type-scoped contexts come into play you should verify the output.

```
  -context string
    	context file
  -document.iri string
    	remote context IRI for this file
  -package.name string
    	Go package name (default "vocab")
```
