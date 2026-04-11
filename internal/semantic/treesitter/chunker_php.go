package treesitter

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/samestrin/llm-tools/internal/semantic"
)

// PHPChunker uses tree-sitter to parse PHP source code.
type PHPChunker struct {
	lang *gotreesitter.Language
}

// NewPHPChunker creates a new tree-sitter based PHP chunker.
func NewPHPChunker() *PHPChunker {
	return &PHPChunker{
		lang: grammars.PhpLanguage(),
	}
}

func (c *PHPChunker) SupportedExtensions() []string {
	return []string{"php", "phtml", "php5", "php7"}
}

func (c *PHPChunker) Chunk(path string, content []byte) ([]semantic.Chunk, error) {
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

func (c *PHPChunker) walkNode(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk, enclosingClass string) {
	WalkNamedChildren(node, c.lang, func(child *gotreesitter.Node, nodeType string) {
		switch nodeType {
		case "function_definition":
			c.extractFunction(child, content, path, chunks, enclosingClass)
		case "class_declaration":
			c.extractClass(child, content, path, chunks)
		case "interface_declaration":
			c.extractInterface(child, content, path, chunks)
		case "trait_declaration":
			c.extractTrait(child, content, path, chunks)
		case "method_declaration":
			c.extractFunction(child, content, path, chunks, enclosingClass)
		case "program":
			// PHP wraps in program node, recurse
			c.walkNode(child, content, path, chunks, enclosingClass)
		}
	})
}

func (c *PHPChunker) extractFunction(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk, enclosingClass string) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	chunkType := semantic.ChunkFunction
	if enclosingClass != "" {
		chunkType = semantic.ChunkMethod
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "php", chunkType, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *PHPChunker) extractClass(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	className := GetNodeText(nameNode, content)
	if className == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "php", semantic.ChunkStruct, className, sig)
	*chunks = append(*chunks, chunk)

	// Extract methods
	bodyNode := GetChildByFieldName(node, "body", c.lang)
	if bodyNode != nil {
		c.walkNode(bodyNode, content, path, chunks, className)
	}
}

func (c *PHPChunker) extractInterface(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "php", semantic.ChunkInterface, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *PHPChunker) extractTrait(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "php", semantic.ChunkStruct, name, sig)
	*chunks = append(*chunks, chunk)

	// Extract methods inside trait
	bodyNode := GetChildByFieldName(node, "body", c.lang)
	if bodyNode != nil {
		c.walkNode(bodyNode, content, path, chunks, name)
	}
}
