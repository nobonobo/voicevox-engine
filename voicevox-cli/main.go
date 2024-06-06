package main

import (
	"bufio"
	"context"
	"log"
	"os"

	engine "github.com/nobonobo/voicevox-engine"
	"golang.org/x/sys/windows"
)

func init() {
	doc, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if err != nil {
		log.Fatal(err)
	}
	engine.Setup(doc)
}

func main() {
	log.Printf("config: %#v", engine.Config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := engine.New(ctx)
	go func() {
		defer func() {
			log.Println("teminated")
			e.Stop()
		}()
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := engine.Clean(scanner.Text())
			if len(line) == 0 {
				return
			}
			e.Speak(line)
		}
	}()
	if err := e.Run(); err != nil {
		log.Fatal(err)
	}
	log.Println("done")
}
