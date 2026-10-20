package asks

// Published six-field unkeyed literals remain source-compatible.
var _ = Question{"example", "Question?", "", nil, false, false}
var _ = Option{"once", true, false}
