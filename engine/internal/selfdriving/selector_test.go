package selfdriving_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/veritone/engine-toolkit/engine/internal/selfdriving"
)

func TestRandomSelector(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	inputDir, cleanup := createTestData(t)
	defer cleanup()
	s := &selfdriving.RandomSelector{
		Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		InputDir:     inputDir,
		InputPattern: "*.txt",
		Logger:       log.New(os.Stdout, "", log.LstdFlags),
	}
	f, err := s.Select(ctx)
	is.NoErr(err)
	defer f.Unlock()
	is.True(f.Path != "")
}

func createTestData(t *testing.T) (string, func()) {
	t.Helper()
	is := is.New(t)
	path := filepath.Join("testdata", time.Now().Format(time.RFC3339Nano))
	f := func() {
		is := is.New(t)
		err := os.RemoveAll(path)
		is.NoErr(err)
	}
	inputDir := filepath.Join(path, "input")
	err := os.MkdirAll(inputDir, 0777)
	is.NoErr(err)
	for i := 0; i < 10; i++ {
		txt := fmt.Sprintf("%d.txt", i)
		err := ioutil.WriteFile(filepath.Join(inputDir, txt), []byte(txt), 0777)
		is.NoErr(err)
	}
	return inputDir, f
}