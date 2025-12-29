package semantic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// MockEmbedder provides deterministic embeddings for benchmarking
type MockEmbedder struct {
	dimensions int
}

func NewMockEmbedder(dims int) *MockEmbedder {
	return &MockEmbedder{dimensions: dims}
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Generate deterministic embedding based on text hash
	embedding := make([]float32, m.dimensions)
	hash := uint32(0)
	for _, ch := range text {
		hash = hash*31 + uint32(ch)
	}
	for i := 0; i < m.dimensions; i++ {
		hash = hash*1103515245 + 12345
		embedding[i] = float32(hash%1000) / 1000.0
	}
	return embedding, nil
}

func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

func (m *MockEmbedder) Dimensions() int {
	return m.dimensions
}

// Sample Go code for benchmarking
const sampleGoCode = `package main

import (
	"fmt"
	"strings"
)

// User represents a user in the system
type User struct {
	ID       int
	Name     string
	Email    string
	IsActive bool
}

// UserService handles user operations
type UserService struct {
	users map[int]*User
}

// NewUserService creates a new user service
func NewUserService() *UserService {
	return &UserService{
		users: make(map[int]*User),
	}
}

// AddUser adds a user to the service
func (s *UserService) AddUser(user *User) error {
	if user.ID <= 0 {
		return fmt.Errorf("invalid user ID")
	}
	s.users[user.ID] = user
	return nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id int) (*User, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// FindByEmail finds users by email substring
func (s *UserService) FindByEmail(substr string) []*User {
	var result []*User
	for _, user := range s.users {
		if strings.Contains(user.Email, substr) {
			result = append(result, user)
		}
	}
	return result
}

// Deactivate deactivates a user
func (s *UserService) Deactivate(id int) error {
	user, err := s.GetUser(id)
	if err != nil {
		return err
	}
	user.IsActive = false
	return nil
}

func main() {
	svc := NewUserService()
	svc.AddUser(&User{ID: 1, Name: "Alice", Email: "alice@example.com", IsActive: true})
	fmt.Println("User service initialized")
}
`

// BenchmarkGoChunker measures chunking performance for Go code
func BenchmarkGoChunker(b *testing.B) {
	chunker := NewGoChunker()
	content := []byte(sampleGoCode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chunker.Chunk("test.go", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSChunker measures chunking performance for JS code
func BenchmarkJSChunker(b *testing.B) {
	chunker := NewJSChunker()
	content := []byte(`
class UserService {
	constructor() {
		this.users = new Map();
	}

	addUser(user) {
		this.users.set(user.id, user);
	}

	getUser(id) {
		return this.users.get(id);
	}

	findByEmail(substr) {
		return Array.from(this.users.values())
			.filter(u => u.email.includes(substr));
	}
}

function createUser(name, email) {
	return { id: Date.now(), name, email, isActive: true };
}

const deactivateUser = (user) => {
	user.isActive = false;
	return user;
};
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chunker.Chunk("test.js", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPythonChunker measures chunking performance for Python code
func BenchmarkPythonChunker(b *testing.B) {
	chunker := NewPythonChunker()
	content := []byte(`
class UserService:
    def __init__(self):
        self.users = {}

    def add_user(self, user):
        self.users[user.id] = user

    def get_user(self, id):
        return self.users.get(id)

    def find_by_email(self, substr):
        return [u for u in self.users.values() if substr in u.email]

def create_user(name, email):
    return {"id": 1, "name": name, "email": email, "is_active": True}

async def fetch_user(user_id):
    response = await http.get(f"/api/users/{user_id}")
    return response.json()
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chunker.Chunk("test.py", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMockEmbedder measures embedding generation performance
func BenchmarkMockEmbedder(b *testing.B) {
	embedder := NewMockEmbedder(1024)
	ctx := context.Background()
	text := "This is a sample function that does something useful in the codebase"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := embedder.Embed(ctx, text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMockEmbedderBatch measures batch embedding performance
func BenchmarkMockEmbedderBatch(b *testing.B) {
	embedder := NewMockEmbedder(1024)
	ctx := context.Background()
	texts := []string{
		"func NewUserService() *UserService",
		"func (s *UserService) AddUser(user *User) error",
		"func (s *UserService) GetUser(id int) (*User, error)",
		"type User struct { ID int; Name string }",
		"type UserService struct { users map[int]*User }",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := embedder.EmbedBatch(ctx, texts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCosineSimilarity measures similarity computation performance
func BenchmarkCosineSimilarity(b *testing.B) {
	dims := 1024
	v1 := make([]float32, dims)
	v2 := make([]float32, dims)
	for i := 0; i < dims; i++ {
		v1[i] = float32(i) / float32(dims)
		v2[i] = float32(dims-i) / float32(dims)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cosineSimilarity(v1, v2)
	}
}

// BenchmarkStorageWrite measures SQLite write performance
func BenchmarkStorageWrite(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	storage, err := NewSQLiteStorage(dbPath, 1024)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()
	embedding := make([]float32, 1024)
	for i := range embedding {
		embedding[i] = float32(i) / 1024.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunk := Chunk{
			ID:        fmt.Sprintf("chunk-%d", i),
			FilePath:  "test.go",
			Type:      ChunkFunction,
			Name:      fmt.Sprintf("Function%d", i),
			Content:   "func test() { return nil }",
			StartLine: 1,
			EndLine:   3,
			Language:  "go",
		}
		err := storage.Create(ctx, chunk, embedding)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearch measures search performance with mock data
func BenchmarkSearch(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	storage, err := NewSQLiteStorage(dbPath, 1024)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()
	embedder := NewMockEmbedder(1024)

	// Seed with chunks
	numChunks := 100
	for i := 0; i < numChunks; i++ {
		chunk := Chunk{
			ID:        fmt.Sprintf("chunk-%d", i),
			FilePath:  fmt.Sprintf("file%d.go", i%10),
			Type:      ChunkFunction,
			Name:      fmt.Sprintf("Function%d", i),
			Content:   fmt.Sprintf("func Function%d() { return %d }", i, i),
			StartLine: i * 10,
			EndLine:   i*10 + 5,
			Language:  "go",
		}
		embedding, _ := embedder.Embed(ctx, chunk.Content)
		if err := storage.Create(ctx, chunk, embedding); err != nil {
			b.Fatal(err)
		}
	}

	searcher := NewSearcher(storage, embedder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searcher.Search(ctx, "find user function", SearchOptions{TopK: 10})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIndexSmallProject benchmarks indexing a small synthetic project
func BenchmarkIndexSmallProject(b *testing.B) {
	// Create a temp directory with sample files
	tmpDir := b.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		b.Fatal(err)
	}

	// Create 10 Go files with sample code
	for i := 0; i < 10; i++ {
		content := fmt.Sprintf(`package main

import "fmt"

type Service%d struct {
	data map[string]interface{}
}

func NewService%d() *Service%d {
	return &Service%d{data: make(map[string]interface{})}
}

func (s *Service%d) Process(input string) (string, error) {
	return fmt.Sprintf("processed: %%s", input), nil
}

func (s *Service%d) Validate(data interface{}) bool {
	return data != nil
}
`, i, i, i, i, i, i)

		filePath := filepath.Join(projectDir, fmt.Sprintf("service%d.go", i))
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
	}

	dbPath := filepath.Join(tmpDir, "index.db")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clean up from previous iteration
		os.Remove(dbPath)

		storage, err := NewSQLiteStorage(dbPath, 1024)
		if err != nil {
			b.Fatal(err)
		}

		embedder := NewMockEmbedder(1024)
		factory := NewChunkerFactory()
		factory.Register("go", NewGoChunker())

		mgr := NewIndexManager(storage, embedder, factory)
		_, err = mgr.Index(context.Background(), projectDir, IndexOptions{})
		if err != nil {
			storage.Close()
			b.Fatal(err)
		}
		storage.Close()
	}
}
