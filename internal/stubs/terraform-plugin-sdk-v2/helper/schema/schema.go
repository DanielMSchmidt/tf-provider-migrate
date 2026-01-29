package schema

import "github.com/hashicorp/terraform-plugin-go/tfprotov5"

type Provider struct {
	Schema         map[string]*Schema
	ResourcesMap   map[string]*Resource
	DataSourcesMap map[string]*Resource
}

func (p *Provider) Meta() interface{} {
	return nil
}

type Resource struct {
	Schema map[string]*Schema
}

type Schema struct {
	Type          ValueType
	Optional      bool
	Required      bool
	Computed      bool
	Sensitive     bool
	Description   string
	Elem          interface{}
	Default       interface{}
	DefaultFunc   interface{}
	Deprecated    string
	ConflictsWith []string
	ExactlyOneOf  []string
	AtLeastOneOf  []string
	RequiredWith  []string
	MinItems      int
	MaxItems      int
}

type ValueType int

const (
	TypeString ValueType = iota
	TypeBool
	TypeInt
	TypeFloat
	TypeList
	TypeSet
	TypeMap
)

func NewGRPCProviderServer(_ *Provider) tfprotov5.ProviderServer {
	return nil
}

func EnvDefaultFunc(_ string, _ interface{}) interface{} {
	return nil
}

func MultiEnvDefaultFunc(_ []string, _ interface{}) interface{} {
	return nil
}
