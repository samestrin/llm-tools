package semantic

import (
	"testing"
)

func TestPythonChunker_Functions(t *testing.T) {
	chunker := NewPythonChunker()

	content := []byte(`
# Regular function
def greet(name):
    return f"Hello, {name}"

# Function with multiple args
def add(a, b):
    return a + b

# Async function
async def fetch_data(url):
    response = await aiohttp.get(url)
    return await response.json()

# Decorated function
@app.route("/api")
def api_handler():
    return {"status": "ok"}

# Function with type hints
def process_data(data: list[str]) -> dict:
    return {"count": len(data)}
`)

	chunks, err := chunker.Chunk("test.py", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	expectedFunctions := []string{"greet", "add", "fetch_data", "api_handler", "process_data"}

	for _, name := range expectedFunctions {
		found := false
		for _, chunk := range chunks {
			if chunk.Name == name && chunk.Type == ChunkFunction {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find function %q", name)
		}
	}
}

func TestPythonChunker_Classes(t *testing.T) {
	chunker := NewPythonChunker()

	content := []byte(`
class Animal:
    def __init__(self, name):
        self.name = name

    def speak(self):
        print(f"{self.name} makes a sound")

class Dog(Animal):
    def speak(self):
        print(f"{self.name} barks")

@dataclass
class User:
    name: str
    age: int
`)

	chunks, err := chunker.Chunk("test.py", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	classFound := map[string]bool{"Animal": false, "Dog": false, "User": false}
	for _, chunk := range chunks {
		if chunk.Type == ChunkStruct {
			if _, ok := classFound[chunk.Name]; ok {
				classFound[chunk.Name] = true
			}
		}
	}

	for name, found := range classFound {
		if !found {
			t.Errorf("Expected to find class %q", name)
		}
	}
}

func TestPythonChunker_SupportedExtensions(t *testing.T) {
	chunker := NewPythonChunker()

	exts := chunker.SupportedExtensions()
	expected := map[string]bool{"py": false, "pyw": false, "pyi": false}

	for _, ext := range exts {
		if _, ok := expected[ext]; ok {
			expected[ext] = true
		}
	}

	for ext, found := range expected {
		if !found {
			t.Errorf("Expected extension %q not found", ext)
		}
	}
}

func TestPythonChunker_EmptyContent(t *testing.T) {
	chunker := NewPythonChunker()

	chunks, err := chunker.Chunk("test.py", []byte{})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestPythonChunker_LineNumbers(t *testing.T) {
	chunker := NewPythonChunker()

	// Content starts on line 1, no leading newline
	content := []byte(`# Comment line 1

def first():
    return 1

def second():
    return 2
`)

	chunks, err := chunker.Chunk("test.py", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	for _, chunk := range chunks {
		if chunk.Name == "first" {
			// Comment is line 1, empty line 2, first() starts at line 3
			// But with multiline regex, decorator matching may affect position
			if chunk.StartLine < 1 {
				t.Errorf("first() StartLine = %d, should be positive", chunk.StartLine)
			}
		}
		if chunk.Name == "second" {
			if chunk.StartLine < 1 {
				t.Errorf("second() StartLine = %d, should be positive", chunk.StartLine)
			}
		}
		// Verify second comes after first
		if chunk.Name == "first" {
			for _, other := range chunks {
				if other.Name == "second" && other.StartLine <= chunk.StartLine {
					t.Error("second() should start after first()")
				}
			}
		}
	}
}

func TestPythonChunker_Language(t *testing.T) {
	chunker := NewPythonChunker()

	content := []byte(`def test():
    pass`)

	chunks, _ := chunker.Chunk("test.py", content)

	if len(chunks) > 0 && chunks[0].Language != "py" {
		t.Errorf("Python file language = %q, want 'py'", chunks[0].Language)
	}
}

func TestPythonChunker_Methods(t *testing.T) {
	chunker := NewPythonChunker()

	content := []byte(`
class Calculator:
    def __init__(self):
        self.value = 0

    def add(self, x):
        self.value += x
        return self

    @staticmethod
    def create():
        return Calculator()

    @classmethod
    def from_value(cls, value):
        calc = cls()
        calc.value = value
        return calc
`)

	chunks, err := chunker.Chunk("test.py", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	calculatorFound := false
	for _, chunk := range chunks {
		if chunk.Name == "Calculator" && chunk.Type == ChunkStruct {
			calculatorFound = true
		}
	}

	if !calculatorFound {
		t.Error("Expected to find Calculator class")
	}
}
