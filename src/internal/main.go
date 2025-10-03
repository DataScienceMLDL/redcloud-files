package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Por ahora solo se guardan en memoria por lo q si ejecutas el programa y luego lo cierras
// se pierde todo lo que tenias en el catalogo aunque en /store quedan los blobs
// TODO : Persistir el catalogo en un archivo y cargarlo al iniciar el programa (es decir sqlLite o
//  un json simple (agregar indices Invertidos para busquedas rapidas))

// main initializes the TBFS (Tag Based File System) CLI application.
// It creates the storage directory, initializes the file catalog, and enters a command loop
// where users can interactively add files with tags, list files by tags, show the catalog contents,
// and delete files by tags. Supported commands are:
//   - add file1 file2 ... --tags tag1,tag2 : Adds files with associated tags to the catalog.
//   - list tag1,tag2                      : Lists files matching the specified tags.
//   - show                                : Displays all entries in the catalog.
//   - delete tag1,tag2                    : Deletes files matching the specified tags from the catalog and storage.
//   - exit                                : Exits the application.


func main() {
	storeDir := "./store"
	os.MkdirAll(storeDir, 0755)

	catalog := NewCatalog()

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Bienvenido a TBFS (Tag Based File System)")
	fmt.Println("Escribe 'exit' para salir")

	for {
		fmt.Print("tbfs> ")
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		if line == "exit" {
			fmt.Println("Exiting...")
			break
		}

		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}

		command := args[0]

		switch command {
		case "add":
			if len(args) < 2 {
				fmt.Println("Usage: add file1 file2 ... --tags tag1,tag2")
				continue
			}

			var fileList []string
			var tags []string

			tagsIndex := -1
			for i, arg := range args {
				if arg == "--tags" {
					tagsIndex = i
					break
				}
			}

			if tagsIndex == -1 {
				fileList = args[1:]
				tags = []string{}
			} else {
				fileList = args[1:tagsIndex]
				if tagsIndex+1 < len(args) {
					tags = strings.Split(args[tagsIndex+1], ",")
				}
			}

			if err := AddFiles(fileList, tags, storeDir, catalog); err != nil {
				fmt.Println("Error:", err)
			}

		case "list":
			if len(args) < 2 {
				fmt.Println("Usage: list tag1,tag2")
				continue
			}
			tags := strings.Split(args[1], ",")
			fmt.Printf("Finding files with tags: %v\n", tags)
			fmt.Printf("Total entries in catalog: %d\n", catalog.Count())
			results := catalog.FindByTags(tags)
			fmt.Printf("Results found: %d\n", len(results))
			for _, entry := range results {
				tagNames := catalog.TagNamesOf(entry)
				fmt.Printf("Blob: %s | File: %s | Tags: %v\n", entry.BlobId, entry.FileName, tagNames)
			}
		case "show":
			fmt.Printf("=== Full catalog contents ===\n")
			fmt.Printf("Total entries: %d\n", catalog.Count())
			allEntries := catalog.GetAllEntries()
			for blobId, entry := range allEntries {
				tagNames := catalog.TagNamesOf(entry)
				fmt.Printf("BlobID: %s | File: %s | Tags: %v\n", blobId, entry.FileName, tagNames)
			}
		case "delete":
			if len(args) < 2 {
				fmt.Println("Usage: delete tag1,tag2")
				continue
			}
			tags := strings.Split(args[1], ",")
			deleted, err := catalog.DeleteByTags(tags, storeDir)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
			for _, entry := range deleted {
				tagNames := catalog.TagNamesOf(entry)
				fmt.Printf("Deleted Blob: %s | File: %s | Tags: %v\n", entry.BlobId, entry.FileName, tagNames)
			}

		default:
			fmt.Println("Unknown command:", command)
		}
	}
}
