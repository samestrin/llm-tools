package semantic

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// Ensure GoChunker implements RefExtractor
var _ RefExtractor = (*GoChunker)(nil)

// ExtractRefs extracts function calls, imports, and type references from Go source code.
func (c *GoChunker) ExtractRefs(path string, content []byte, chunks []Chunk) ([]ChunkRef, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, 0)
	if err != nil {
		return nil, err
	}

	// Build a set of chunk IDs by name for quick lookup
	chunkByName := make(map[string]string, len(chunks))
	for _, ch := range chunks {
		chunkByName[ch.Name] = ch.ID
	}

	var refs []ChunkRef

	// Extract imports
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		// Use the last segment as the ref name (e.g., "fmt" from "fmt", "http" from "net/http")
		parts := strings.Split(importPath, "/")
		refName := parts[len(parts)-1]
		if imp.Name != nil && imp.Name.Name != "." && imp.Name.Name != "_" {
			refName = imp.Name.Name
		}
		// Associate import with all chunks in this file
		for _, ch := range chunks {
			refs = append(refs, ChunkRef{
				ChunkID: ch.ID,
				RefType: RefImports,
				RefName: refName,
			})
			break // Only associate with first chunk to avoid bloat
		}
	}

	// Walk AST for function calls and type references per chunk
	for _, ch := range chunks {
		startLine := ch.StartLine
		endLine := ch.EndLine

		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				return false
			}

			pos := fset.Position(n.Pos())
			if pos.Line < startLine || pos.Line > endLine {
				return true // outside this chunk, keep walking
			}

			switch expr := n.(type) {
			case *ast.CallExpr:
				callName := extractCallName(expr)
				if callName != "" && callName != ch.Name {
					refs = append(refs, ChunkRef{
						ChunkID: ch.ID,
						RefType: RefCalls,
						RefName: callName,
					})
				}

			case *ast.SelectorExpr:
				// Type references like pkg.Type
				if ident, ok := expr.X.(*ast.Ident); ok {
					typeName := ident.Name + "." + expr.Sel.Name
					refs = append(refs, ChunkRef{
						ChunkID: ch.ID,
						RefType: RefUsesType,
						RefName: typeName,
					})
				}
			}

			return true
		})
	}

	return refs, nil
}

// extractCallName extracts the function/method name from a call expression.
func extractCallName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if ident, ok := fn.X.(*ast.Ident); ok {
			return ident.Name + "." + fn.Sel.Name
		}
		return fn.Sel.Name
	}
	return ""
}
