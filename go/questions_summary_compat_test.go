package asks

// Published six-field unkeyed literals remain source-compatible.
var _ = Question{"example", "Question?", "", nil, false, false}

// Option aliases catalogue.Option. A local defined view keeps the positional
// field-count/type check while vet treats the deliberate literal as local.
type publishedOptionLayout Option

var _ = Option(publishedOptionLayout{"once", true, false})
