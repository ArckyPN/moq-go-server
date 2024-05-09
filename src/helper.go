package main

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

func cwd() (p string) {
	var (
		err   error
		parts []string
	)
	if p, err = os.Getwd(); err != nil {
		log.Fatal("Error: reading cwd")
	}

	parts = strings.Split(p, "/")
	p = strings.Join(parts[:len(parts)-2], "/")

	return
}

func getDirectoryOfFile(path string) string {
	var (
		parts []string = strings.Split(path, "/")
	)

	parts = parts[:len(parts)-1]
	return strings.Join(parts, "/")
}

func createPathToFile(path string) (err error) {
	var (
		dir string = getDirectoryOfFile(path)
	)

	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Printf("Error: %s\n", err)
		return
	}

	return
}

func createFile(path string) (fp *os.File, err error) {
	if err = createPathToFile(path); err != nil {
		log.Printf("Error: %s\n", err)
		return
	}

	if fp, err = os.Create(path); err != nil {
		log.Printf("Error: %s\n", err)
		return
	}

	return
}

func clearQlogDirectory() (err error) {
	var (
		path string = fmt.Sprintf("%s/data/qlog", cwd())
	)

	err = os.RemoveAll(path)

	return
}
