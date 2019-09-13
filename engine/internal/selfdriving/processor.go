package selfdriving

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Processor struct {
	Logger     *log.Logger
	Selector   FileSelector
	Process    func(outputFile string, file File) error
	MoveToDir  string
	ErrToDir   string
	ResultsDir string
}

func (p *Processor) Run(ctx context.Context) error {
	if err := os.MkdirAll(p.MoveToDir, 0777); err != nil {
		p.Logger.Println("failed to make output directory:", err)
	}
	if err := os.MkdirAll(p.ErrToDir, 0777); err != nil {
		p.Logger.Println("failed to make output errors directory:", err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := func() error {
			file, err := p.Selector.Select(ctx)
			if err != nil {
				return err
			}
			defer file.Unlock()
			if err := p.processFile(file); err != nil {
				p.Logger.Println("ERROR: process:", err)
				if errMove := file.Move(p.ErrToDir); errMove != nil {
					p.Logger.Println("ERROR: move to the error dir:", errMove)
				} else {
					p.Logger.Printf("moved file with errors to %s\n", file.Path)
					// write the error to a .error file to be able to inspect
					file.WriteErr(err)
				}
				return nil
			}
			if err := file.Move(p.MoveToDir); err != nil {
				p.Logger.Println("ERROR: move file:", err)
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
}

func (p *Processor) processFile(file File) error {
	now := time.Now()
	dir := filepath.Join(p.ResultsDir, now.Format("2006-01"), now.Format("20060102-1500"))
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	outputFile := filepath.Join(dir, filepath.Base(file.Path)+".json")
	return p.Process(outputFile, file)
}