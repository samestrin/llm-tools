package semantic

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// GoChunker implements the Chunker interface for Go source code
type GoChunker struct{}

// NewGoChunker creates a new Go AST-based chunker
func NewGoChunker() *GoChunker {
	return &GoChunker{}
}

// Chunk parses Go source code and extracts semantic chunks
func (c *GoChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	var chunks []Chunk

	// Walk the AST and extract chunks
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			chunk := c.extractFunction(fset, node, content)
			chunk.FilePath = path
			chunk.Language = "go"
			chunks = append(chunks, chunk)
			return false // Don't recurse into function body

		case *ast.GenDecl:
			// Handle type declarations (structs, interfaces)
			if node.Tok == token.TYPE {
				for _, spec := range node.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						chunk := c.extractType(fset, node, ts, content)
						chunk.FilePath = path
						chunk.Language = "go"
						chunks = append(chunks, chunk)
					}
				}
			}
			return false
		}
		return true
	})

	return chunks, nil
}

// SupportedExtensions returns the file extensions this chunker handles
func (c *GoChunker) SupportedExtensions() []string {
	return []string{"go"}
}

// extractFunction extracts a function or method chunk from an AST node
func (c *GoChunker) extractFunction(fset *token.FileSet, fn *ast.FuncDecl, content []byte) Chunk {
	chunk := Chunk{
		Name:      fn.Name.Name,
		StartLine: fset.Position(fn.Pos()).Line,
		EndLine:   fset.Position(fn.End()).Line,
	}

	// Determine if this is a method or function
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		chunk.Type = ChunkMethod
	} else {
		chunk.Type = ChunkFunction
	}

	// Build signature
	chunk.Signature = c.buildFuncSignature(fn)

	// Extract content (including doc comments if present)
	startPos := fn.Pos()
	if fn.Doc != nil {
		startPos = fn.Doc.Pos()
		chunk.StartLine = fset.Position(startPos).Line
	}

	startOffset := fset.Position(startPos).Offset
	endOffset := fset.Position(fn.End()).Offset
	if startOffset >= 0 && endOffset <= len(content) {
		chunk.Content = string(content[startOffset:endOffset])
	}

	chunk.ID = chunk.GenerateID()
	return chunk
}

// extractType extracts a struct or interface chunk from an AST node
func (c *GoChunker) extractType(fset *token.FileSet, decl *ast.GenDecl, ts *ast.TypeSpec, content []byte) Chunk {
	chunk := Chunk{
		Name:      ts.Name.Name,
		StartLine: fset.Position(ts.Pos()).Line,
		EndLine:   fset.Position(ts.End()).Line,
	}

	// Determine type kind
	switch ts.Type.(type) {
	case *ast.InterfaceType:
		chunk.Type = ChunkInterface
	case *ast.StructType:
		chunk.Type = ChunkStruct
	default:
		chunk.Type = ChunkFunction // fallback for type aliases, etc.
	}

	// Build signature
	chunk.Signature = "type " + ts.Name.Name

	// Extract content (including doc comments)
	startPos := ts.Pos()
	if decl.Doc != nil {
		startPos = decl.Doc.Pos()
		chunk.StartLine = fset.Position(startPos).Line
	} else if ts.Doc != nil {
		startPos = ts.Doc.Pos()
		chunk.StartLine = fset.Position(startPos).Line
	}

	startOffset := fset.Position(startPos).Offset
	endOffset := fset.Position(ts.End()).Offset
	if startOffset >= 0 && endOffset <= len(content) {
		chunk.Content = string(content[startOffset:endOffset])
	}

	chunk.ID = chunk.GenerateID()
	return chunk
}

// buildFuncSignature creates a readable function signature
func (c *GoChunker) buildFuncSignature(fn *ast.FuncDecl) string {
	var buf bytes.Buffer

	buf.WriteString("func ")

	// Add receiver if present
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		buf.WriteString("(")
		recv := fn.Recv.List[0]
		if len(recv.Names) > 0 {
			buf.WriteString(recv.Names[0].Name)
			buf.WriteString(" ")
		}
		buf.WriteString(c.typeToString(recv.Type))
		buf.WriteString(") ")
	}

	buf.WriteString(fn.Name.Name)

	// Add parameters
	buf.WriteString("(")
	if fn.Type.Params != nil {
		buf.WriteString(c.fieldsToString(fn.Type.Params.List))
	}
	buf.WriteString(")")

	// Add return types
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		results := c.fieldsToString(fn.Type.Results.List)
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			buf.WriteString(" ")
			buf.WriteString(results)
		} else {
			buf.WriteString(" (")
			buf.WriteString(results)
			buf.WriteString(")")
		}
	}

	return buf.String()
}

// fieldsToString converts a field list to a string representation
func (c *GoChunker) fieldsToString(fields []*ast.Field) string {
	var parts []string

	for _, field := range fields {
		typeStr := c.typeToString(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, typeStr)
		} else {
			names := make([]string, len(field.Names))
			for i, name := range field.Names {
				names[i] = name.Name
			}
			parts = append(parts, strings.Join(names, ", ")+" "+typeStr)
		}
	}

	return strings.Join(parts, ", ")
}

// typeToString converts an AST type expression to a string
func (c *GoChunker) typeToString(expr ast.Expr) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), expr)
	return buf.String()
}
