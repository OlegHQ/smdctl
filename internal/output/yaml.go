package output

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// FormatYAML formats data as YAML and prints it
func FormatYAML(data interface{}) error {
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	fmt.Println(string(out))
	return nil
}
