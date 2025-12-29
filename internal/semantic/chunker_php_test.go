package semantic

import (
	"testing"
)

func TestPHPChunker_Functions(t *testing.T) {
	chunker := NewPHPChunker()

	content := []byte(`<?php

function greet($name) {
    return "Hello, " . $name;
}

function add($a, $b): int {
    return $a + $b;
}

public function publicFunc() {
    return true;
}

private function privateFunc() {
    return false;
}

protected static function staticFunc() {
    return null;
}
`)

	chunks, err := chunker.Chunk("test.php", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	expectedFunctions := []string{"greet", "add", "publicFunc", "privateFunc", "staticFunc"}

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

func TestPHPChunker_Classes(t *testing.T) {
	chunker := NewPHPChunker()

	content := []byte(`<?php

class Animal {
    public function speak() {
        echo "Animal speaks";
    }
}

class Dog extends Animal {
    public function speak() {
        echo "Dog barks";
    }
}

abstract class Vehicle {
    abstract public function move();
}

final class Car extends Vehicle {
    public function move() {
        echo "Car drives";
    }
}
`)

	chunks, err := chunker.Chunk("test.php", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	classFound := map[string]bool{"Animal": false, "Dog": false, "Vehicle": false, "Car": false}
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

func TestPHPChunker_Interfaces(t *testing.T) {
	chunker := NewPHPChunker()

	content := []byte(`<?php

interface Moveable {
    public function move(): void;
}

interface Speakable {
    public function speak(): string;
}

trait Loggable {
    public function log($message) {
        echo $message;
    }
}
`)

	chunks, err := chunker.Chunk("test.php", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	expectedInterfaces := map[string]bool{"Moveable": false, "Speakable": false}
	for _, chunk := range chunks {
		if chunk.Type == ChunkInterface {
			if _, ok := expectedInterfaces[chunk.Name]; ok {
				expectedInterfaces[chunk.Name] = true
			}
		}
	}

	for name, found := range expectedInterfaces {
		if !found {
			t.Errorf("Expected to find interface %q", name)
		}
	}
}

func TestPHPChunker_SupportedExtensions(t *testing.T) {
	chunker := NewPHPChunker()

	exts := chunker.SupportedExtensions()
	expected := map[string]bool{"php": false, "phtml": false, "php5": false, "php7": false}

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

func TestPHPChunker_EmptyContent(t *testing.T) {
	chunker := NewPHPChunker()

	chunks, err := chunker.Chunk("test.php", []byte{})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestPHPChunker_Language(t *testing.T) {
	chunker := NewPHPChunker()

	content := []byte(`<?php
function test() {
    return true;
}
`)

	chunks, _ := chunker.Chunk("test.php", content)

	if len(chunks) > 0 && chunks[0].Language != "php" {
		t.Errorf("PHP file language = %q, want 'php'", chunks[0].Language)
	}
}
