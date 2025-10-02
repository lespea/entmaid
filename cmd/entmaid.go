package cmd

import (
	"fmt"
	"os"
	"strings"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func GenerateDiagram(schemaPath string, targetPath string, outputType OutputType, startPattern string, endPattern string) error {
	graph, err := entc.LoadGraph(schemaPath, &gen.Config{})
	if err != nil {
		return fmt.Errorf("failed to load schema graph from the path %s: %v", schemaPath, err)
	}
	// Generate the Mermaid code for the ERD diagram
	mermaidCode, err := generateMermaidCode(graph)
	if err != nil {
		return err
	}

	mermaidCode = addMermaidToType(mermaidCode, outputType)

	err = insertMultiLineString(targetPath, mermaidCode, startPattern, endPattern)
	if err != nil {
		return fmt.Errorf("failed to insert Mermaid code into the file: %v", err)
	}

	fmt.Println("Mermaid file generated successfully.")

	return nil
}

// generateMermaidCode generates the Mermaid code for the ERD diagram based on the schema graph.
func generateMermaidCode(graph *gen.Graph) (string, error) {
	var builder strings.Builder

	builder.WriteString("erDiagram\n")

	for _, node := range graph.Nodes {
		builder.WriteString(fmt.Sprintf(" %s {\n", node.Name))

		if node.HasOneFieldID() {
			builder.WriteString(fmt.Sprintf("  %s %s PK\n", formatType(node.ID.Type.String()), node.ID.Name))
		}

		for _, field := range node.Fields {
			builder.WriteString(fmt.Sprintf("  %s %s\n", formatType(field.Type.String()), field.Name))
		}

		for _, foreignKey := range node.ForeignKeys {
			// For now we don't support user defined foreign keys as need to test them out more.
			// Will add support for them in the future and focus on the ent generated ones.
			if foreignKey.UserDefined {
				continue
			}

			builder.WriteString(fmt.Sprintf("  %s %s FK\n", formatType(foreignKey.Field.Type.String()), foreignKey.Field.Name))
		}

		builder.WriteString(" }\n\n")

		for _, edge := range node.Edges {
			// Ent handles M2M relationships in a way that we can't easily generate an accurate ERD with it.
			// SO we attempt to extract out the actual M2M table to properly display it.
			if edge.M2M() {
				// We need to map the relationship between both base tables, but only create the table once.
				if !edge.IsInverse() {
					rel := edge.Rel
					builder.WriteString(fmt.Sprintf(" %s {\n", rel.Table))

					for _, column := range rel.Columns {
						builder.WriteString(fmt.Sprintf("  %s %s PK,FK\n", "int", column))
					}

					builder.WriteString(" }\n\n")
				}
			}
		}
	}

	for _, node := range graph.Nodes {
		for _, edge := range node.Edges {
			// Need to handle M2M relationships a bit more special.
			if edge.M2M() {
				builder.WriteString(fmt.Sprintf(" %s %s %s : %s%s\n", node.Name, "|o--o{", edge.Rel.Table, edge.Name, getEdgeRefName(edge.Ref)))
				continue
			}

			if edge.IsInverse() {
				continue
			}

			_, err := builder.WriteString(fmt.Sprintf(" %s %s %s : %s%s\n", node.Name, getEdgeRelationship(edge), edge.Type.Name, edge.Name, getEdgeRefName(edge.Ref)))
			if err != nil {
				return "", fmt.Errorf("failed to write string: %v", err)
			}
		}
	}

	return builder.String(), nil
}

func addMermaidToType(mermaidCode string, outputType OutputType) string {
	switch outputType {
	case Markdown:
		return fmt.Sprintf("```mermaid\n%s\n```", mermaidCode)
	case Plain:
		return mermaidCode
	default:
		return mermaidCode
	}
}

func formatType(s string) string {
	switch s {
	case "time.Time":
		return "timestamp"

	case "map[string]interface {}", "map[string]interface{}", "map[string]any":
		return "jsonb"

	default:
		return strings.ReplaceAll(s, ".", "-")
	}
}

func getEdgeRelationship(edge *gen.Edge) string {
	if edge.O2M() {
		return "|o--o{"
	}

	if edge.M2O() {
		return "}o--o|"
	}

	if edge.M2M() {
		return "}o--o{"
	}

	return "|o--o|"
}

func getEdgeRefName(ref *gen.Edge) string {
	if ref == nil {
		return ""
	}

	return fmt.Sprintf("-%s", ref.Name)
}

func insertMultiLineString(filePath string, multiLineString string, startPattern string, endPattern string) error {
	// Read the content of the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Convert the content to a string
	fileContent := string(content)

	// Find the starting and ending strings
	startIndex := strings.Index(fileContent, startPattern)
	endIndex := strings.Index(fileContent, endPattern)

	// Check if the starting and ending strings are found
	if startIndex == -1 || endIndex == -1 {
		return fmt.Errorf("starting (%s) or ending (%s) string not found in the file", startPattern, endPattern)
	}

	// Construct the updated content with the generated multi-line string
	updatedContent := fileContent[:startIndex+len(startPattern)+1] + multiLineString + "\n" + fileContent[endIndex:]

	// Write the updated content back to the file
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		return err
	}

	return nil
}
