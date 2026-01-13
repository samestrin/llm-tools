package semantic

import (
	"strings"
	"testing"
)

func TestRustChunker_Functions(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
fn simple_function() {
    println!("Hello");
}

pub fn public_function(x: i32) -> i32 {
    x * 2
}

async fn async_function() -> Result<(), Error> {
    Ok(())
}

pub async fn pub_async_function() {
    // do something
}

unsafe fn unsafe_function() {
    // unsafe code
}
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Should find all 5 functions
	funcCount := 0
	for _, chunk := range chunks {
		if chunk.Type == ChunkFunction {
			funcCount++
		}
	}

	if funcCount != 5 {
		t.Errorf("Expected 5 functions, got %d", funcCount)
		for _, chunk := range chunks {
			t.Logf("Chunk: %s %s (type: %s)", chunk.Type, chunk.Name, chunk.Signature)
		}
	}

	// Check specific function names
	expected := map[string]bool{
		"simple_function":    false,
		"public_function":    false,
		"async_function":     false,
		"pub_async_function": false,
		"unsafe_function":    false,
	}

	for _, chunk := range chunks {
		if chunk.Type == ChunkFunction {
			if _, ok := expected[chunk.Name]; ok {
				expected[chunk.Name] = true
			}
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("Expected to find function %s", name)
		}
	}
}

func TestRustChunker_Structs(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
struct Point {
    x: i32,
    y: i32,
}

pub struct PublicPoint {
    pub x: i32,
    pub y: i32,
}

struct TupleStruct(i32, i32);

pub(crate) struct CrateVisible {
    field: String,
}
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	structCount := 0
	for _, chunk := range chunks {
		if chunk.Type == ChunkStruct {
			structCount++
		}
	}

	if structCount != 4 {
		t.Errorf("Expected 4 structs, got %d", structCount)
		for _, chunk := range chunks {
			t.Logf("Chunk: %s %s", chunk.Type, chunk.Name)
		}
	}
}

func TestRustChunker_Enums(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
enum Color {
    Red,
    Green,
    Blue,
}

pub enum Option<T> {
    Some(T),
    None,
}
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	enumCount := 0
	for _, chunk := range chunks {
		if chunk.Type == ChunkStruct && chunk.Signature == "enum "+chunk.Name {
			enumCount++
		}
	}

	if enumCount != 2 {
		t.Errorf("Expected 2 enums, got %d", enumCount)
	}
}

func TestRustChunker_Traits(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
trait Drawable {
    fn draw(&self);
}

pub trait Serializable {
    fn serialize(&self) -> Vec<u8>;
    fn deserialize(data: &[u8]) -> Self;
}

unsafe trait UnsafeTrait {
    fn do_unsafe(&self);
}
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	traitCount := 0
	for _, chunk := range chunks {
		if chunk.Type == ChunkInterface {
			traitCount++
		}
	}

	if traitCount != 3 {
		t.Errorf("Expected 3 traits, got %d", traitCount)
	}
}

func TestRustChunker_Impls(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
struct Point {
    x: i32,
    y: i32,
}

impl Point {
    fn new(x: i32, y: i32) -> Self {
        Point { x, y }
    }

    fn distance(&self) -> f64 {
        ((self.x * self.x + self.y * self.y) as f64).sqrt()
    }
}

impl Default for Point {
    fn default() -> Self {
        Point { x: 0, y: 0 }
    }
}

impl<T> Clone for MyStruct<T> {
    fn clone(&self) -> Self {
        // clone implementation
    }
}
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	implCount := 0
	for _, chunk := range chunks {
		if chunk.Type == ChunkMethod {
			implCount++
		}
	}

	if implCount != 3 {
		t.Errorf("Expected 3 impl blocks, got %d", implCount)
		for _, chunk := range chunks {
			t.Logf("Chunk: %s %s sig=%s", chunk.Type, chunk.Name, chunk.Signature)
		}
	}
}

func TestRustChunker_Modules(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
mod utils;

pub mod helpers {
    pub fn help() {}
}

mod tests {
    #[test]
    fn test_something() {
        assert!(true);
    }
}
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	modCount := 0
	for _, chunk := range chunks {
		if chunk.Type == ChunkFile {
			modCount++
		}
	}

	if modCount != 3 {
		t.Errorf("Expected 3 modules, got %d", modCount)
	}
}

func TestRustChunker_TypeAliases(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
type Result<T> = std::result::Result<T, Error>;

pub type BoxedFuture<T> = Box<dyn Future<Output = T>>;
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	typeCount := 0
	for _, chunk := range chunks {
		if chunk.Type == ChunkStruct && (chunk.Name == "Result" || chunk.Name == "BoxedFuture") {
			typeCount++
		}
	}

	if typeCount != 2 {
		t.Errorf("Expected 2 type aliases, got %d", typeCount)
	}
}

func TestRustChunker_ComplexFile(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
//! A complex Rust file for testing

use std::io::{Read, Write};

/// A point in 2D space
#[derive(Debug, Clone)]
pub struct Point {
    pub x: f64,
    pub y: f64,
}

impl Point {
    /// Creates a new point
    pub fn new(x: f64, y: f64) -> Self {
        Point { x, y }
    }

    /// Calculates distance from origin
    pub fn distance(&self) -> f64 {
        (self.x * self.x + self.y * self.y).sqrt()
    }
}

impl Default for Point {
    fn default() -> Self {
        Point::new(0.0, 0.0)
    }
}

/// A drawable trait
pub trait Drawable {
    fn draw(&self);
}

impl Drawable for Point {
    fn draw(&self) {
        println!("Point at ({}, {})", self.x, self.y);
    }
}

/// Result type alias
pub type PointResult = Result<Point, String>;

/// Helper function
pub fn create_point(x: f64, y: f64) -> PointResult {
    if x.is_nan() || y.is_nan() {
        Err("Invalid coordinates".to_string())
    } else {
        Ok(Point::new(x, y))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_point_creation() {
        let p = Point::new(3.0, 4.0);
        assert_eq!(p.distance(), 5.0);
    }
}
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) < 5 {
		t.Errorf("Expected at least 5 chunks, got %d", len(chunks))
	}

	// Log all chunks for debugging
	for _, chunk := range chunks {
		t.Logf("Found: %s '%s' at lines %d-%d", chunk.Type, chunk.Name, chunk.StartLine, chunk.EndLine)
	}
}

func TestRustChunker_SupportedExtensions(t *testing.T) {
	chunker := NewRustChunker()
	exts := chunker.SupportedExtensions()

	if len(exts) != 1 || exts[0] != "rs" {
		t.Errorf("Expected [rs], got %v", exts)
	}
}

func TestRustChunker_EmptyContent(t *testing.T) {
	chunker := NewRustChunker()
	chunks, err := chunker.Chunk("test.rs", []byte{})

	if err != nil {
		t.Errorf("Expected no error for empty content, got %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestRustChunker_GenericFunctions(t *testing.T) {
	chunker := NewRustChunker()

	content := []byte(`
fn generic_fn<T>(x: T) -> T {
    x
}

pub fn generic_with_bounds<T: Clone + Debug>(x: T) -> T {
    x.clone()
}

fn multiple_generics<T, U>(x: T, y: U) -> (T, U) {
    (x, y)
}
`)

	chunks, err := chunker.Chunk("test.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	funcCount := 0
	for _, chunk := range chunks {
		if chunk.Type == ChunkFunction {
			funcCount++
		}
	}

	if funcCount != 3 {
		t.Errorf("Expected 3 generic functions, got %d", funcCount)
	}
}

func TestRustChunker_EdgeCases(t *testing.T) {
	chunker := NewRustChunker()

	// Test extractContent edge cases via the Chunk method
	t.Run("single line file", func(t *testing.T) {
		content := []byte(`fn single() {}`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		if len(chunks) != 1 {
			t.Errorf("Expected 1 chunk, got %d", len(chunks))
		}
	})

	t.Run("struct without body", func(t *testing.T) {
		content := []byte(`struct Unit;`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		if len(chunks) != 1 {
			t.Errorf("Expected 1 chunk, got %d", len(chunks))
		}
	})

	t.Run("extern fn", func(t *testing.T) {
		content := []byte(`
extern "C" fn external_function() {
    // C-compatible function
}
`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		funcFound := false
		for _, chunk := range chunks {
			if chunk.Type == ChunkFunction && chunk.Name == "external_function" {
				funcFound = true
			}
		}
		if !funcFound {
			t.Error("Expected to find external_function")
		}
	})

	t.Run("pub(crate) visibility", func(t *testing.T) {
		content := []byte(`
pub(crate) fn crate_visible() {}
pub(super) fn super_visible() {}
pub(in crate::module) fn path_visible() {}
`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		if len(chunks) < 3 {
			t.Errorf("Expected at least 3 functions, got %d", len(chunks))
		}
	})

	t.Run("impl with generics", func(t *testing.T) {
		content := []byte(`
impl<T> MyStruct<T> {
    fn new() -> Self {
        Self {}
    }
}
`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		implFound := false
		for _, chunk := range chunks {
			if chunk.Type == ChunkMethod {
				implFound = true
			}
		}
		if !implFound {
			t.Error("Expected to find impl block")
		}
	})

	t.Run("duplicate names in different contexts", func(t *testing.T) {
		content := []byte(`
struct Foo {}
impl Foo {
    fn new() -> Self { Foo {} }
}
fn new() -> i32 { 42 }
`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		// Should find struct, impl, and standalone function
		if len(chunks) < 3 {
			t.Errorf("Expected at least 3 chunks, got %d", len(chunks))
		}
	})

	t.Run("unclosed block", func(t *testing.T) {
		content := []byte(`
fn unclosed() {
    // This block is never closed
`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		// Should still extract the function
		if len(chunks) != 1 {
			t.Errorf("Expected 1 chunk, got %d", len(chunks))
		}
	})

	t.Run("no block started", func(t *testing.T) {
		content := []byte(`
mod external_module;
use std::io;
`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		// Should find the module declaration
		modFound := false
		for _, chunk := range chunks {
			if chunk.Type == ChunkFile && chunk.Name == "external_module" {
				modFound = true
			}
		}
		if !modFound {
			t.Error("Expected to find external_module")
		}
	})
}

func TestRustChunker_ExtractContentEdgeCases(t *testing.T) {
	chunker := NewRustChunker()

	// Test with content that exercises boundary conditions
	t.Run("very long function", func(t *testing.T) {
		// Create a function with many lines
		var sb strings.Builder
		sb.WriteString("fn long_function() {\n")
		for i := 0; i < 100; i++ {
			sb.WriteString("    let x = " + string(rune('0'+i%10)) + ";\n")
		}
		sb.WriteString("}\n")

		content := []byte(sb.String())
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		if len(chunks) != 1 {
			t.Errorf("Expected 1 chunk, got %d", len(chunks))
		}
		if chunks[0].EndLine-chunks[0].StartLine < 100 {
			t.Errorf("Expected function to span 100+ lines")
		}
	})

	t.Run("deeply nested braces", func(t *testing.T) {
		content := []byte(`
fn nested() {
    if true {
        if true {
            if true {
                let x = 1;
            }
        }
    }
}
`)
		chunks, err := chunker.Chunk("test.rs", content)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		if len(chunks) != 1 {
			t.Errorf("Expected 1 chunk, got %d", len(chunks))
		}
	})
}

func TestRustChunker_HelperFunctions(t *testing.T) {
	chunker := NewRustChunker()

	t.Run("extractSignature with invalid index", func(t *testing.T) {
		lines := []string{"fn test() {}", "}"}
		// Test negative index
		sig := chunker.extractSignature(lines, -1)
		if sig != "" {
			t.Errorf("Expected empty signature for negative index, got %s", sig)
		}
		// Test out of bounds
		sig = chunker.extractSignature(lines, 100)
		if sig != "" {
			t.Errorf("Expected empty signature for out of bounds index, got %s", sig)
		}
	})

	t.Run("extractContent with inverted range", func(t *testing.T) {
		lines := []string{"line1", "line2", "line3"}
		content := chunker.extractContent(lines, 2, 0)
		if content != "" {
			t.Errorf("Expected empty content for inverted range, got %s", content)
		}
	})

	t.Run("extractContent with negative start", func(t *testing.T) {
		lines := []string{"line1", "line2", "line3"}
		content := chunker.extractContent(lines, -5, 1)
		if content != "line1\nline2" {
			t.Errorf("Expected 'line1\\nline2', got %s", content)
		}
	})

	t.Run("extractContent with end beyond bounds", func(t *testing.T) {
		lines := []string{"line1", "line2", "line3"}
		content := chunker.extractContent(lines, 1, 100)
		if content != "line2\nline3" {
			t.Errorf("Expected 'line2\\nline3', got %s", content)
		}
	})

	t.Run("lineNumber", func(t *testing.T) {
		content := "line1\nline2\nline3"
		line := chunker.lineNumber(content, 0)
		if line != 1 {
			t.Errorf("Expected line 1, got %d", line)
		}
		line = chunker.lineNumber(content, 6)
		if line != 2 {
			t.Errorf("Expected line 2, got %d", line)
		}
		line = chunker.lineNumber(content, 12)
		if line != 3 {
			t.Errorf("Expected line 3, got %d", line)
		}
	})

	t.Run("findBlockEnd with no braces", func(t *testing.T) {
		lines := []string{"mod foo;", "use bar;"}
		end := chunker.findBlockEnd(lines, 0)
		if end != 1 {
			t.Errorf("Expected end at 1, got %d", end)
		}
	})
}
