package selfdriving_test

import (
	"context"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/veritone/engine-toolkit/engine/selfdriving"
)

func TestProcessing(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	inputDir, cleanup := createTestData(t)
	defer cleanup()
	processFunc := func(f selfdriving.File) error {
		log.Println("TODO: process", f.Path)
		time.Sleep(1 * time.Second)
		return nil
	}
	s := &selfdriving.RandomSelector{
		Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		InputDir:     inputDir,
		InputPattern: "*.txt",
		Logger:       log.New(os.Stdout, "", log.LstdFlags),
	}
	outputDir := filepath.Join(filepath.Dir(inputDir), "output")
	p := &selfdriving.Processor{
		Files:     s,
		Logger:    log.New(os.Stdout, "", log.LstdFlags),
		OutputDir: outputDir,
		Process:   processFunc,
	}
	if err := p.Run(ctx); err != nil {
		t.Logf("run: %v", err)
	}
}

func TestProcessingPipeline(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	inputDir, cleanup := createTestData(t)
	defer cleanup()

	dir := filepath.Dir(inputDir)
	output1Dir := filepath.Join(dir, "2")
	output2Dir := filepath.Join(dir, "3")
	output3Dir := filepath.Join(dir, "output")
	is.NoErr(os.MkdirAll(output1Dir, 0777))
	is.NoErr(os.MkdirAll(output2Dir, 0777))
	is.NoErr(os.MkdirAll(output3Dir, 0777))

	s1 := &selfdriving.RandomSelector{
		PollInterval:      100 * time.Millisecond,
		Rand:              rand.New(rand.NewSource(time.Now().UnixNano())),
		InputDir:          inputDir,
		InputPattern:      "*.txt",
		Logger:            log.New(os.Stdout, "", log.LstdFlags),
		WaitForReadyFiles: false,
	}
	s2 := &selfdriving.RandomSelector{
		PollInterval:      100 * time.Millisecond,
		Rand:              rand.New(rand.NewSource(time.Now().UnixNano())),
		InputDir:          output1Dir,
		InputPattern:      "*.txt",
		Logger:            log.New(os.Stdout, "", log.LstdFlags),
		WaitForReadyFiles: true,
	}
	s3 := &selfdriving.RandomSelector{
		PollInterval:      100 * time.Millisecond,
		Rand:              rand.New(rand.NewSource(time.Now().UnixNano())),
		InputDir:          output2Dir,
		InputPattern:      "*.txt",
		Logger:            log.New(os.Stdout, "", log.LstdFlags),
		WaitForReadyFiles: true,
	}

	p1 := &selfdriving.Processor{
		Files:     s1,
		Logger:    log.New(os.Stdout, "", log.LstdFlags),
		OutputDir: output1Dir,
		Process: func(f selfdriving.File) error {
			log.Println("TODO: process 1:", f.Path)
			time.Sleep(250 * time.Millisecond)
			return nil
		},
	}
	p2 := &selfdriving.Processor{
		Files:     s2,
		Logger:    log.New(os.Stdout, "", log.LstdFlags),
		OutputDir: output2Dir,
		Process: func(f selfdriving.File) error {
			log.Println("TODO: process 2:", f.Path)
			time.Sleep(250 * time.Millisecond)
			return nil
		},
	}
	p3 := &selfdriving.Processor{
		Files:     s3,
		Logger:    log.New(os.Stdout, "", log.LstdFlags),
		OutputDir: output3Dir,
		Process: func(f selfdriving.File) error {
			log.Println("TODO: process 3:", f.Path)
			time.Sleep(250 * time.Millisecond)
			return nil
		},
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p1.Run(ctx); err != nil {
			t.Logf("run: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p2.Run(ctx); err != nil {
			t.Logf("run: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p3.Run(ctx); err != nil {
			t.Logf("run: %v", err)
		}
	}()

	wg.Wait()

}
