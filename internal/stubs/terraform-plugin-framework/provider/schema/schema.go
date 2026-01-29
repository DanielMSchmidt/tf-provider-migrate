package schema

import "github.com/hashicorp/terraform-plugin-framework/types"

type Schema struct {
	Attributes map[string]Attribute
	Blocks     map[string]Block
}

type Block interface{}

type Attribute interface{}

type StringAttribute struct {
	Optional    bool
	Required    bool
	Computed    bool
	Sensitive   bool
	Description string
}

type BoolAttribute struct {
	Optional    bool
	Required    bool
	Computed    bool
	Sensitive   bool
	Description string
}

type Int64Attribute struct {
	Optional    bool
	Required    bool
	Computed    bool
	Sensitive   bool
	Description string
}

type Float64Attribute struct {
	Optional    bool
	Required    bool
	Computed    bool
	Sensitive   bool
	Description string
}

type ListAttribute struct {
	Optional    bool
	Required    bool
	Computed    bool
	Sensitive   bool
	Description string
	ElementType types.Type
}

type SetAttribute struct {
	Optional    bool
	Required    bool
	Computed    bool
	Sensitive   bool
	Description string
	ElementType types.Type
}

type MapAttribute struct {
	Optional    bool
	Required    bool
	Computed    bool
	Sensitive   bool
	Description string
	ElementType types.Type
}
