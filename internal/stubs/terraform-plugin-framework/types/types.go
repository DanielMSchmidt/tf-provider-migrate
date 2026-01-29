package types

type Type interface{}

type baseType struct{}

var (
	StringType  Type = baseType{}
	BoolType    Type = baseType{}
	Int64Type   Type = baseType{}
	Float64Type Type = baseType{}
)
