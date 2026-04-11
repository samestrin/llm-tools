package treesitter

import (
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/samestrin/llm-tools/internal/semantic"
)

// PythonChunker uses tree-sitter to parse Python source code into semantic chunks.
type PythonChunker struct {
	lang *gotreesitter.Language
}

// NewPythonChunker creates a new tree-sitter based Python chunker.
func NewPythonChunker() *PythonChunker {
	return &PythonChunker{
		lang: grammars.PythonLanguage(),
	}
}

func (c *PythonChunker) SupportedExtensions() []string {
	return []string{"py", "pyw", "pyi"}
}

func (c *PythonChunker) Chunk(path string, content []byte) ([]semantic.Chunk, error) {
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

func (c *PythonChunker) walkNode(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk, enclosingClass string) {
	WalkNamedChildren(node, c.lang, func(child *gotreesitter.Node, nodeType string) {
		switch nodeType {
		case "function_definition":
			c.extractFunction(child, content, path, chunks, enclosingClass)

		case "class_definition":
			c.extractClass(child, content, path, chunks)

		case "decorated_definition":
			// Look at the definition inside the decorator
			defNode := GetChildByFieldName(child, "definition", c.lang)
			if defNode != nil {
				defType := defNode.Type(c.lang)
				switch defType {
				case "function_definition":
					c.extractFunction(child, content, path, chunks, enclosingClass)
				case "class_definition":
					c.extractClass(defNode, content, path, chunks)
				}
			}
		}
	})
}

func (c *PythonChunker) extractFunction(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk, enclosingClass string) {
	// For decorated_definition, the actual function is inside
	funcNode := node
	if node.Type(c.lang) == "decorated_definition" {
		if defNode := GetChildByFieldName(node, "definition", c.lang); defNode != nil {
			funcNode = defNode
		}
	}

	nameNode := GetChildByFieldName(funcNode, "name", c.lang)
	name := GetNodeText(nameNode, content)
	if name == "" {
		return
	}

	chunkType := semantic.ChunkFunction
	if enclosingClass != "" {
		chunkType = semantic.ChunkMethod
	}

	sig := ExtractSignature(funcNode, content)

	chunk := NodeToChunk(node, content, path, "py", chunkType, name, sig)
	*chunks = append(*chunks, chunk)
}

func (c *PythonChunker) extractClass(node *gotreesitter.Node, content []byte, path string, chunks *[]semantic.Chunk) {
	nameNode := GetChildByFieldName(node, "name", c.lang)
	className := GetNodeText(nameNode, content)
	if className == "" {
		return
	}

	sig := ExtractSignature(node, content)
	chunk := NodeToChunk(node, content, path, "py", semantic.ChunkStruct, className, sig)
	*chunks = append(*chunks, chunk)

	// Also extract methods inside the class body
	bodyNode := GetChildByFieldName(node, "body", c.lang)
	if bodyNode != nil {
		c.walkNode(bodyNode, content, path, chunks, className)
	}
}
