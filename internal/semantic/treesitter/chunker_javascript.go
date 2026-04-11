package treesitter

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/samestrin/llm-tools/internal/semantic"
)

// JSChunker uses tree-sitter to parse JavaScript and TypeScript source code.
type JSChunker struct {
	jsLang *gotreesitter.Language
	tsLang *gotreesitter.Language
}

// NewJSChunker creates a new tree-sitter based JavaScript/TypeScript chunker.
func NewJSChunker() *JSChunker {
	return &JSChunker{
		jsLang: grammars.JavascriptLanguage(),
		tsLang: grammars.TypescriptLanguage(),
	}
}

func (c *JSChunker) SupportedExtensions() []string {
	return []string{"js", "jsx", "ts", "tsx", "mjs", "cjs"}
}

func (c *JSChunker) langForExt(ext string) *gotreesitter.Language {
	switch ext {
	case "ts", "tsx":
		return c.tsLang
	default:
		return c.jsLang
	}
}

func (c *JSChunker) Chunk(path string, content []byte) ([]semantic.Chunk, error) {
	ext := semantic.LanguageFromExtension(path)
	lang := c.langForExt(ext)

	tree, err := ParseSource(content, lang)
	if err != nil {
		return nil, err
	}
	defer tree.Release()

	var chunks []semantic.Chunk
	root := tree.RootNode()

	c.walkNode(root, lang, content, path, ext, &chunks, "")
	return chunks, nil
}

func (c *JSChunker) walkNode(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte, path, ext string, chunks *[]semantic.Chunk, enclosingClass string) {
	WalkNamedChildren(node, lang, func(child *gotreesitter.Node, nodeType string) {
		switch nodeType {
		case "function_declaration":
			c.extractFunction(child, lang, content, path, ext, chunks, enclosingClass)

		case "class_declaration":
			c.extractClass(child, lang, content, path, ext, chunks)

		case "interface_declaration":
			c.extractInterface(child, lang, content, path, ext, chunks)

		case "type_alias_declaration":
			c.extractTypeAlias(child, lang, content, path, ext, chunks)

		case "lexical_declaration":
			// Check for arrow functions: const foo = () => {}
			c.extractArrowFunctions(child, lang, content, path, ext, chunks, enclosingClass)

		case "export_statement":
			// Recurse into exported declarations
			c.walkNode(child, lang, content, path, ext, chunks, enclosingClass)
		}
	})
}

func (c *JSChunker) extractFunction(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte, path, ext string, chunks *[]semantic.Chunk, enclosingClass string) {
	nameNode := GetChildByFieldName(node, "name", lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	chunkType := semantic.ChunkFunction
	if enclosingClass != "" {
		chunkType = semantic.ChunkMethod
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, ext, chunkType, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *JSChunker) extractClass(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte, path, ext string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", lang)
	className := GetNodeText(nameNode, content)
	if className == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, ext, semantic.ChunkStruct, className, sig)
	*chunks = append(*chunks, chunk)

	// Extract methods inside class body
	bodyNode := GetChildByFieldName(node, "body", lang)
	if bodyNode != nil {
		WalkNamedChildren(bodyNode, lang, func(child *gotreesitter.Node, nodeType string) {
			switch nodeType {
			case "method_definition":
				c.extractMethod(child, lang, content, path, ext, chunks, className)
			case "public_field_definition", "field_definition":
				// Skip fields
			}
		})
	}
}

func (c *JSChunker) extractMethod(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte, path, ext string, chunks *[]semantic.Chunk, className string) {
	nameNode := GetChildByFieldName(node, "name", lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, ext, semantic.ChunkMethod, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *JSChunker) extractInterface(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte, path, ext string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, ext, semantic.ChunkInterface, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *JSChunker) extractTypeAlias(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte, path, ext string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, ext, semantic.ChunkStruct, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *JSChunker) extractArrowFunctions(node *gotreesitter.Node, lang *gotreesitter.Language, content []byte, path, ext string, chunks *[]semantic.Chunk, enclosingClass string) {
	// lexical_declaration > variable_declarator > arrow_function
	WalkNamedChildren(node, lang, func(child *gotreesitter.Node, nodeType string) {
		if nodeType == "variable_declarator" {
			nameNode := GetChildByFieldName(child, "name", lang)
			valueNode := GetChildByFieldName(child, "value", lang)
			if nameNode != nil && valueNode != nil {
				valueType := valueNode.Type(lang)
				if valueType == "arrow_function" || valueType == "function_expression" || valueType == "function" {
					name := GetNodeText(nameNode, content)
					if name == "" {
						return
					}

					chunkType := semantic.ChunkFunction
					if enclosingClass != "" {
						chunkType = semantic.ChunkMethod
					}

					// Use the parent lexical_declaration as the full node for content
					sig := ExtractSignature(node, content)
					chunk := NodeToChunk(node, content, path, ext, chunkType, name, sig)
					*chunks = append(*chunks, chunk)
				}
			}
		}
	})
}
