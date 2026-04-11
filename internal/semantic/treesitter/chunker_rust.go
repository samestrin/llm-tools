package treesitter

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/samestrin/llm-tools/internal/semantic"
)

// RustChunker uses tree-sitter to parse Rust source code.
type RustChunker struct {
	lang *gotreesitter.Language
}

// NewRustChunker creates a new tree-sitter based Rust chunker.
func NewRustChunker() *RustChunker {
	return &RustChunker{
		lang: grammars.RustLanguage(),
	}
}

func (c *RustChunker) SupportedExtensions() []string {
	return []string{"rs"}
}

func (c *RustChunker) Chunk(path string, content []byte) ([]semantic.Chunk, error) {
	tree, err := ParseSource(content, c.lang)
	if err != nil {
		return nil, err
	}
	defer tree.Release()

	var chunks []semantic.Chunk
	root := tree.RootNode()

	c.walkNode(root, content, path, &chunks, "")
	return chunks, nil
}

func (c *RustChunker) walkNode(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk, enclosingType string) {
	WalkNamedChildren(node, c.lang, func(child *gotreesitter.Node, nodeType string) {
		switch nodeType {
		case "function_item":
			c.extractFunction(child, content, path, chunks, enclosingType)
		case "struct_item":
			c.extractStruct(child, content, path, chunks)
		case "enum_item":
			c.extractEnum(child, content, path, chunks)
		case "trait_item":
			c.extractTrait(child, content, path, chunks)
		case "impl_item":
			c.extractImpl(child, content, path, chunks)
		case "mod_item":
			c.extractMod(child, content, path, chunks)
		case "type_item":
			c.extractTypeAlias(child, content, path, chunks)
		}
	})
}

func (c *RustChunker) extractFunction(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk, enclosingType string) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	chunkType := semantic.ChunkFunction
	if enclosingType != "" {
		chunkType = semantic.ChunkMethod
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "rs", chunkType, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *RustChunker) extractStruct(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "rs", semantic.ChunkStruct, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *RustChunker) extractEnum(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "rs", semantic.ChunkStruct, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *RustChunker) extractTrait(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "rs", semantic.ChunkInterface, name, sig)
	*chunks = append(*chunks, chunk)

	// Extract methods inside trait
	bodyNode := GetChildByFieldName(node, "body", c.lang)
	if bodyNode != nil {
		c.walkNode(bodyNode, content, path, chunks, name)
	}
}

func (c *RustChunker) extractImpl(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	// impl blocks: extract the type name and optional trait
	typeNode := GetChildByFieldName(node, "type", c.lang)
	typeName := GetNodeText(typeNode, content)
	if typeName == "" {
		return
	}

	// Check for trait: impl Trait for Type
	traitNode := GetChildByFieldName(node, "trait", c.lang)
	traitName := GetNodeText(traitNode, content)

	implName := typeName
	if traitName != "" {
		implName = typeName + "_" + traitName
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "rs", semantic.ChunkStruct, implName, sig)
	*chunks = append(*chunks, chunk)

	// Extract methods inside impl
	bodyNode := GetChildByFieldName(node, "body", c.lang)
	if bodyNode != nil {
		c.walkNode(bodyNode, content, path, chunks, typeName)
	}
}

func (c *RustChunker) extractMod(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	// Only extract modules with a body (inline modules), not declarations
	bodyNode := GetChildByFieldName(node, "body", c.lang)
	if bodyNode == nil {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "rs", semantic.ChunkFile, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *RustChunker) extractTypeAlias(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "rs", semantic.ChunkStruct, name, sig)
	*chunks = append(*chunks, chunk)
}
