package fraudindex

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

func LoadJSONReferences(path string) ([]Reference, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open references: %w", err)
	}
	defer file.Close()

	var raw []struct {
		Vector Vector `json:"vector"`
		Label  string `json:"label"`
	}
	if err := json.NewDecoder(file).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode references: %w", err)
	}

	references := make([]Reference, 0, len(raw))
	for i, item := range raw {
		label, err := ParseLabel(item.Label)
		if err != nil {
			return nil, fmt.Errorf("reference %d: %w", i, err)
		}
		references = append(references, Reference{
			Vector: item.Vector,
			Label:  label,
		})
	}
	return references, nil
}

func StreamJSONReferences(reader io.Reader, handle func(Reference) error) (uint64, error) {
	decoder := json.NewDecoder(reader)

	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("read references opening token: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		return 0, errors.New("references JSON must start with array")
	}

	var count uint64
	for decoder.More() {
		var raw struct {
			Vector Vector `json:"vector"`
			Label  string `json:"label"`
		}
		if err := decoder.Decode(&raw); err != nil {
			return count, fmt.Errorf("decode reference %d: %w", count, err)
		}
		label, err := ParseLabel(raw.Label)
		if err != nil {
			return count, fmt.Errorf("reference %d: %w", count, err)
		}
		if err := handle(Reference{Vector: raw.Vector, Label: label}); err != nil {
			return count, fmt.Errorf("handle reference %d: %w", count, err)
		}
		count++
	}

	token, err = decoder.Token()
	if err != nil {
		return count, fmt.Errorf("read references closing token: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != ']' {
		return count, errors.New("references JSON must end with array")
	}

	return count, nil
}

func ParseLabel(label string) (Label, error) {
	switch label {
	case "legit":
		return LabelLegit, nil
	case "fraud":
		return LabelFraud, nil
	default:
		return 0, errors.New("unknown label " + label)
	}
}
