package file

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestNewWriterConfigBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		options      []Option
		wantContains string
	}{
		{name: "empty filename", wantContains: "filename"},
		{name: "zero max size", filename: "app.log", options: []Option{WithMaxSize(0)}, wantContains: "maxSize"},
		{name: "negative max size", filename: "app.log", options: []Option{WithMaxSize(-1)}, wantContains: "maxSize"},
		{name: "negative max backups", filename: "app.log", options: []Option{WithMaxBackups(-1)}, wantContains: "maxBackups"},
		{name: "negative max age", filename: "app.log", options: []Option{WithMaxAge(-1)}, wantContains: "maxAge"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := NewWriter(tt.filename, tt.options...)
			if err == nil {
				_ = writer.Close()
				t.Fatal("NewWriter() error = nil; want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Fatalf("NewWriter() error = %q; want contain %q", err, tt.wantContains)
			}
		})
	}
}

func TestNewWriter_RejectsWhenParentIsFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewWriter(filepath.Join(parent, "app.log"))
	if err == nil {
		t.Fatal("NewWriter() error = nil; want mkdir or open error")
	}
}

func TestNewWriterUniquifiesFilename(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	writer, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	assertUniquifiedFilename(t, filename, writer.Filename)

	other, err := NewWriter(filepath.Join(filepath.Dir(filename), "other.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	if other.Filename == writer.Filename {
		t.Fatalf("different stems produced the same Filename %q", writer.Filename)
	}
}

func TestNewWriterUniquifiesFilenameWithoutExtension(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app")
	writer, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	assertUniquifiedFilename(t, filename, writer.Filename)
}

func TestNewWriterSamePathUsesSameUniquifiedName(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	first, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if first == second {
		t.Fatal("same path returned the same writer instance; want distinct instances")
	}
	if first.Filename != second.Filename {
		t.Fatalf("Filename = %q, %q; want the same uniquified path", first.Filename, second.Filename)
	}
}

func TestNewWriterDefaults(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	writer, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	assertUniquifiedFilename(t, filename, writer.Filename)
	if writer.MaxSize != 100 {
		t.Fatalf("MaxSize = %d; want 100", writer.MaxSize)
	}
	if writer.MaxBackups != 0 {
		t.Fatalf("MaxBackups = %d; want 0", writer.MaxBackups)
	}
	if writer.MaxAge != 0 {
		t.Fatalf("MaxAge = %d; want 0", writer.MaxAge)
	}
	if writer.Compress {
		t.Fatal("Compress = true; want false")
	}
	if !writer.LocalTime {
		t.Fatal("LocalTime = false; want true")
	}
}

func TestNewWriterAppliesOptions(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	writer, err := NewWriter(
		filename,
		nil,
		WithMaxSize(1),
		WithMaxBackups(2),
		WithMaxAge(3),
		WithCompress(true),
		WithLocalTime(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	assertUniquifiedFilename(t, filename, writer.Filename)
	if writer.MaxSize != 1 {
		t.Fatalf("MaxSize = %d; want 1", writer.MaxSize)
	}
	if writer.MaxBackups != 2 {
		t.Fatalf("MaxBackups = %d; want 2", writer.MaxBackups)
	}
	if writer.MaxAge != 3 {
		t.Fatalf("MaxAge = %d; want 3", writer.MaxAge)
	}
	if !writer.Compress {
		t.Fatal("Compress = false; want true")
	}
	if !writer.LocalTime {
		t.Fatal("LocalTime = false; want true")
	}
}

func TestNewWriter_LastOptionWins(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	writer, err := NewWriter(
		filename,
		WithMaxSize(1),
		WithMaxSize(50),
		WithMaxBackups(1),
		WithMaxBackups(3),
		WithMaxAge(1),
		WithMaxAge(9),
		WithCompress(true),
		WithCompress(false),
		WithLocalTime(true),
		WithLocalTime(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	if writer.MaxSize != 50 {
		t.Fatalf("MaxSize = %d; want 50", writer.MaxSize)
	}
	if writer.MaxBackups != 3 {
		t.Fatalf("MaxBackups = %d; want 3", writer.MaxBackups)
	}
	if writer.MaxAge != 9 {
		t.Fatalf("MaxAge = %d; want 9", writer.MaxAge)
	}
	if writer.Compress {
		t.Fatal("Compress = true; want false")
	}
	if writer.LocalTime {
		t.Fatal("LocalTime = true; want false")
	}
}

func TestWriterWritesImmediately(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	writer, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	if _, err := writer.Write([]byte("written immediately")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(writer.Filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "written immediately" {
		t.Fatalf("file content = %q; want %q", data, "written immediately")
	}
}

func TestWriterRotate(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "app.log")
	writer, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	if _, err := writer.Write([]byte("before rotate")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Rotate(); err != nil {
		t.Fatal(err)
	}

	activeData, err := os.ReadFile(writer.Filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeData) != 0 {
		t.Fatalf("active file content = %q; want empty after rotation", activeData)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var backupData []byte
	for _, entry := range entries {
		if entry.Name() == filepath.Base(writer.Filename) {
			continue
		}
		if backupData != nil {
			t.Fatalf("found multiple backup files in %s", directory)
		}
		backupData, err = os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
	}
	if string(backupData) != "before rotate" {
		t.Fatalf("backup file content = %q; want %q", backupData, "before rotate")
	}
}

func TestWriterClose(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	writer, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := writer.Write([]byte("closed")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("repeated Close() error = %v; want nil", err)
	}

	data, err := os.ReadFile(writer.Filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "closed" {
		t.Fatalf("file content = %q; want %q", data, "closed")
	}
}

func TestWriterConcurrentWrite(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	writer, err := NewWriter(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	const writes = 256
	start := make(chan struct{})
	errs := make(chan error, writes)
	var wait sync.WaitGroup
	for range writes {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := writer.Write([]byte("line\n"))
			errs <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(writer.Filename)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "line\n"); got != writes {
		t.Fatalf("line count = %d; want %d", got, writes)
	}
}

func TestWriterConcurrentWriteAndRotate(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "app.log")
	writer, err := NewWriter(filename, WithMaxBackups(8))
	if err != nil {
		t.Fatal(err)
	}

	const writers = 32
	const writesPer = 16
	start := make(chan struct{})
	errs := make(chan error, writers*writesPer+1)
	var wait sync.WaitGroup
	for range writers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for range writesPer {
				_, err := writer.Write([]byte("line\n"))
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wait.Add(1)
	go func() {
		defer wait.Done()
		<-start
		errs <- writer.Rotate()
	}()
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	got := 0
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		got += strings.Count(string(data), "line\n")
	}
	if got != writers*writesPer {
		t.Fatalf("line count = %d; want %d", got, writers*writesPer)
	}
}

func assertUniquifiedFilename(t *testing.T, original, got string) {
	t.Helper()
	if got == original {
		t.Fatalf("Filename = %q; want uniquified path", got)
	}
	if filepath.Dir(got) != filepath.Dir(original) {
		t.Fatalf("Filename dir = %q; want %q", filepath.Dir(got), filepath.Dir(original))
	}
	ext := filepath.Ext(original)
	stem := strings.TrimSuffix(filepath.Base(original), ext)
	base := filepath.Base(got)
	if !strings.HasPrefix(base, stem+"-") {
		t.Fatalf("Filename base = %q; want prefix %q", base, stem+"-")
	}
	if !strings.HasSuffix(base, ext) {
		t.Fatalf("Filename base = %q; want suffix %q", base, ext)
	}
	if !strings.Contains(base, strconv.Itoa(os.Getpid())) {
		t.Fatalf("Filename base = %q; want contain pid %d", base, os.Getpid())
	}
}
