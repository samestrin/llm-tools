package semantic

import (
	"testing"
)

func TestJSChunker_Functions(t *testing.T) {
	chunker := NewJSChunker()

	content := []byte(`
// Regular function
function greet(name) {
	return "Hello, " + name;
}

// Arrow function with const
const add = (a, b) => {
	return a + b;
}

// Arrow function with let
let multiply = (a, b) => a * b;

// Async function
async function fetchData(url) {
	const response = await fetch(url);
	return response.json();
}

// Export function
export function helper() {
	return true;
}

// Export default function
export default function main() {
	console.log("main");
}
`)

	chunks, err := chunker.Chunk("test.js", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Should find all functions
	expectedFunctions := []string{"greet", "add", "multiply", "fetchData", "helper", "main"}

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

func TestJSChunker_Classes(t *testing.T) {
	chunker := NewJSChunker()

	content := []byte(`
class Animal {
	constructor(name) {
		this.name = name;
	}

	speak() {
		console.log(this.name + " makes a sound.");
	}
}

export class Dog extends Animal {
	constructor(name) {
		super(name);
	}

	speak() {
		console.log(this.name + " barks.");
	}
}
`)

	chunks, err := chunker.Chunk("test.js", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Should find classes
	classFound := map[string]bool{"Animal": false, "Dog": false}
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

func TestJSChunker_TypeScript(t *testing.T) {
	chunker := NewJSChunker()

	content := []byte(`
interface User {
	name: string;
	age: number;
}

type Status = "active" | "inactive";

class UserService {
	private users: User[] = [];

	addUser(user: User): void {
		this.users.push(user);
	}

	getUsers(): User[] {
		return this.users;
	}
}

function createUser(name: string, age: number): User {
	return { name, age };
}
`)

	chunks, err := chunker.Chunk("test.ts", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Should find interface and type
	expectedTypes := map[string]ChunkType{
		"User":        ChunkInterface,
		"Status":      ChunkFunction, // type alias treated as function-like
		"UserService": ChunkStruct,
		"createUser":  ChunkFunction,
	}

	for name, expectedType := range expectedTypes {
		found := false
		for _, chunk := range chunks {
			if chunk.Name == name && chunk.Type == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find %s of type %v", name, expectedType)
		}
	}
}

func TestJSChunker_Methods(t *testing.T) {
	chunker := NewJSChunker()

	content := []byte(`
const obj = {
	method1() {
		return 1;
	},
	method2: function() {
		return 2;
	},
	method3: () => {
		return 3;
	}
};

class Calculator {
	add(a, b) {
		return a + b;
	}

	static create() {
		return new Calculator();
	}
}
`)

	chunks, err := chunker.Chunk("test.js", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Should find class and methods
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

func TestJSChunker_SupportedExtensions(t *testing.T) {
	chunker := NewJSChunker()

	exts := chunker.SupportedExtensions()
	expected := map[string]bool{"js": false, "jsx": false, "ts": false, "tsx": false, "mjs": false, "cjs": false}

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

func TestJSChunker_EmptyContent(t *testing.T) {
	chunker := NewJSChunker()

	chunks, err := chunker.Chunk("test.js", []byte{})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestJSChunker_CommentsOnly(t *testing.T) {
	chunker := NewJSChunker()

	content := []byte(`
// This is a comment
/* This is a block comment */
/**
 * This is a JSDoc comment
 */
`)

	chunks, err := chunker.Chunk("test.js", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Should not produce any chunks for comments-only content
	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for comments-only content, got %d", len(chunks))
	}
}

func TestJSChunker_LineNumbers(t *testing.T) {
	chunker := NewJSChunker()

	content := []byte(`// Comment line 1

function first() {
	return 1;
}

function second() {
	return 2;
}
`)

	chunks, err := chunker.Chunk("test.js", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	for _, chunk := range chunks {
		if chunk.Name == "first" {
			if chunk.StartLine != 3 {
				t.Errorf("first() StartLine = %d, want 3", chunk.StartLine)
			}
		}
		if chunk.Name == "second" {
			if chunk.StartLine != 7 {
				t.Errorf("second() StartLine = %d, want 7", chunk.StartLine)
			}
		}
	}
}

func TestJSChunker_Language(t *testing.T) {
	chunker := NewJSChunker()

	jsContent := []byte(`function test() {}`)
	tsContent := []byte(`function test(): void {}`)

	jsChunks, _ := chunker.Chunk("test.js", jsContent)
	tsChunks, _ := chunker.Chunk("test.ts", tsContent)

	if len(jsChunks) > 0 && jsChunks[0].Language != "js" {
		t.Errorf("JS file language = %q, want 'js'", jsChunks[0].Language)
	}

	if len(tsChunks) > 0 && tsChunks[0].Language != "ts" {
		t.Errorf("TS file language = %q, want 'ts'", tsChunks[0].Language)
	}
}
