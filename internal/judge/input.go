package judge

// mapInput implements runner.GeneratorInput for in-process strop calls.
type mapInput struct {
	fields  map[string]interface{}
	version int
}

func newMapInput(fields map[string]interface{}, version int) mapInput {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	return mapInput{fields: fields, version: version}
}

func (m mapInput) ToMap() map[string]interface{} { return m.fields }
func (m mapInput) GetVersion() int               { return m.version }
