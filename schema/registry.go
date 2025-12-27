package schema

import (
	"fmt"
	"sync"
)

var (
	globalRegistry = &registry{
		schemas: make(map[string]*Schema),
	}
)

type registry struct {
	mu      sync.RWMutex
	schemas map[string]*Schema
}

func (r *registry) register(schema *Schema) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.schemas[schema.name]; exists {
		return fmt.Errorf("schema %s already registered", schema.name)
	}

	r.schemas[schema.name] = schema
	return nil
}

func (r *registry) get(name string) (*Schema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schema, ok := r.schemas[name]
	return schema, ok
}

func (r *registry) list() []*Schema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schemas := make([]*Schema, 0, len(r.schemas))
	for _, schema := range r.schemas {
		schemas = append(schemas, schema)
	}
	return schemas
}

func RegisterSchema(schema *Schema) error {
	return globalRegistry.register(schema)
}

func GetSchema(name string) (*Schema, bool) {
	return globalRegistry.get(name)
}

func ListSchemas() []*Schema {
	return globalRegistry.list()
}
